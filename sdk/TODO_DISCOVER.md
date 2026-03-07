# TODO: `discover` Module — All SDKs

## What `discover` does

Scans the filesystem for `holon.yaml` files and builds an inventory
of available holons. This is the same logic `op discover` uses, but
embedded in the SDK so holons can find each other without `op`.

## API (same shape in every language)

```
discover(root: string) → []HolonEntry

HolonEntry:
  slug:          string          # given_name-family_name
  uuid:          string          # from holon.yaml
  dir:           string          # absolute path to holon directory
  relative_path: string          # relative to root
  origin:        string          # "local" | "$OPBIN" | "cache"
  identity:      Identity        # parsed from holon.yaml
  manifest:      Manifest | null # parsed build/artifacts section
```

## Scan rules (same as `op`)

1. Walk recursively from `root`.
2. Skip directories: `.git`, `.op`, `node_modules`, `vendor`,
   `build`, and any directory starting with `.`.
3. When `holon.yaml` is found, parse it and add to results.
4. UUID dedup: if same UUID appears twice, keep the one closest
   to root (shallowest depth).
5. Search order: effective local root → `$OPBIN` → `$OPPATH/cache/`.

## Implementation per SDK

### Tier 1 — SDKs with recipe consumers (implement first)

#### `go-holons` → `pkg/discover/discover.go`

```go
func Discover(root string) ([]HolonEntry, error)
func DiscoverLocal() ([]HolonEntry, error)  // root = cwd
func DiscoverAll() ([]HolonEntry, error)    // local + $OPBIN + cache
func FindBySlug(slug string) (*HolonEntry, error)
func FindByUUID(prefix string) (*HolonEntry, error)
```

- Use `filepath.WalkDir` with skip logic.
- Parse `holon.yaml` using existing `identity` package from
  `sophia-who/pkg/identity`.
- Add `pkg/discover/` alongside existing `pkg/serve/` and
  `pkg/transport/`.
- Tests: create temp tree with nested holons, verify scan,
  exclusions, UUID dedup.

#### `rust-holons` → `src/discover.rs`

```rust
pub fn discover(root: &Path) -> Result<Vec<HolonEntry>>
pub fn discover_local() -> Result<Vec<HolonEntry>>
pub fn discover_all() -> Result<Vec<HolonEntry>>
pub fn find_by_slug(slug: &str) -> Result<Option<HolonEntry>>
pub fn find_by_uuid(prefix: &str) -> Result<Option<HolonEntry>>
```

- Use `walkdir` crate (or `std::fs::read_dir` recursive).
- Parse `holon.yaml` with `serde_yaml` (already a dependency).
- Tests: `#[test]` with `tempdir`.

#### `dart-holons` → `lib/src/discover.dart`

```dart
Future<List<HolonEntry>> discover(String root);
Future<List<HolonEntry>> discoverLocal();
Future<HolonEntry?> findBySlug(String slug);
```

- Use `dart:io` `Directory.list(recursive: true)`.
- Parse YAML with `package:yaml`.

#### `swift-holons` → `Sources/Holons/Discover.swift`

```swift
public struct HolonEntry { ... }
public func discover(root: URL) throws -> [HolonEntry]
public func discoverLocal() throws -> [HolonEntry]
public func findBySlug(_ slug: String) throws -> HolonEntry?
```

- Use `FileManager.default.enumerator(at:)`.
- Parse YAML with `Yams` SPM package (or manual parsing).

### Tier 2 — SDKs with hello-world examples

#### `js-holons` → `src/discover.js`

```javascript
export async function discover(root) → HolonEntry[]
export async function findBySlug(slug) → HolonEntry | null
```

- Use `fs.promises.readdir` recursive.
- Parse YAML with `js-yaml` package.

#### `js-web-holons` → `src/discover.mjs`

**Note**: browser JS cannot scan filesystems. This module should
provide `discoverFromManifest(url)` instead — fetch a manifest
from a known endpoint. The full filesystem `discover()` is only
available in Node.js contexts.

#### `kotlin-holons` → `src/main/kotlin/holons/Discover.kt`

```kotlin
fun discover(root: Path): List<HolonEntry>
fun findBySlug(slug: String): HolonEntry?
```

- Use `java.nio.file.Files.walk()`.
- Parse YAML with `snakeyaml` or `kaml`.

#### `csharp-holons` → `Holons/Discover.cs`

```csharp
public static List<HolonEntry> Discover(string root)
public static HolonEntry? FindBySlug(string slug)
```

- Use `Directory.EnumerateFiles` recursive.
- Parse YAML with `YamlDotNet`.

#### `python-holons` → `holons/discover.py`

```python
def discover(root: str) -> list[HolonEntry]
def discover_local() -> list[HolonEntry]
def find_by_slug(slug: str) -> HolonEntry | None
```

- Use `os.walk()`.
- Parse YAML with `pyyaml`.

#### `ruby-holons` → `lib/holons/discover.rb`

```ruby
module Holons
  def self.discover(root) → [HolonEntry]
  def self.find_by_slug(slug) → HolonEntry?
end
```

- Use `Dir.glob("**/holon.yaml")`.
- Parse YAML with stdlib `yaml`.

### Tier 3 — Native SDKs (C, C++, Obj-C)

#### `c-holons` → `src/discover.c` + `include/holons/discover.h`

```c
int holons_discover(const char *root, holon_entry_t **entries, size_t *count);
holon_entry_t *holons_find_by_slug(const char *slug);
void holons_free_entries(holon_entry_t *entries, size_t count);
```

- Use `opendir`/`readdir` recursive.
- Parse YAML with a lightweight C YAML parser (libyaml) or
  line-by-line extraction for the few required fields.

#### `cpp-holons` → `include/holons/discover.hpp`

```cpp
namespace holons {
  std::vector<HolonEntry> discover(const std::filesystem::path& root);
  std::optional<HolonEntry> findBySlug(const std::string& slug);
}
```

- Use `std::filesystem::recursive_directory_iterator`.
- Parse YAML with `yaml-cpp`.

#### `objc-holons` → `src/Discover.m` + `include/Holons/Discover.h`

```objc
+ (NSArray<HolonEntry *> *)discoverInRoot:(NSString *)root;
+ (HolonEntry *)findBySlug:(NSString *)slug;
```

- Use `NSFileManager enumeratorAtURL:`.

#### `java-holons` → `src/main/java/holons/Discover.java`

```java
public static List<HolonEntry> discover(Path root)
public static Optional<HolonEntry> findBySlug(String slug)
```

- Use `java.nio.file.Files.walk()`.
- Parse YAML with `snakeyaml`.

## Testing pattern

For every SDK:

1. Create a temp directory tree:
   ```
   tmp/
   ├── holon-a/holon.yaml     (uuid-a, slug: alice-engine)
   ├── nested/holon-b/holon.yaml (uuid-b, slug: bob-worker)
   ├── .git/holon.yaml        (should be SKIPPED)
   ├── node_modules/x/holon.yaml (should be SKIPPED)
   └── dup/holon-a/holon.yaml (uuid-a again, should be DEDUPED)
   ```
2. Call `discover(tmp)`.
3. Assert: 2 entries (alice-engine, bob-worker).
4. Assert: `.git` and `node_modules` entries not found.
5. Assert: duplicate uuid-a kept only once (shallowest).
6. Call `findBySlug("alice-engine")` → found.
7. Call `findBySlug("nonexistent")` → not found.
