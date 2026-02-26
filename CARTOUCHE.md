---
# Cartouche v1
title: "Cartouche — Document Metadata Specification"
author:
  name: "B. ALTER & Claude"
  copyright: "© 2026 Benoit Pereira da Silva"
created: 2026-02-09
revised: 2026-02-09
lang: en-US
origin_lang: en-US
translation_of: null
translator: null
access:
  humans: true
  agents: true
status: review
---

# Cartouche — Document Metadata Specification

**Version**: 1  
**Copyright**: © 2026 Benoit Pereira da Silva

---

## Purpose

A **Cartouche** is a YAML frontmatter block placed at the top of every Markdown
document in an Organic Programming project. It answers five questions at a glance:

1. **Who** wrote this document?
2. **When** was it created and last revised?
3. **Who** is allowed to read it?
4. **What language** was it thought and written in?
5. **What is its maturity?**

The name is borrowed from the cartouches of Egyptian hieroglyphs — a bounded
frame that identifies and protects the content it encloses.

---

## Format

The cartouche is a standard YAML frontmatter block delimited by `---`.
A comment on the first line identifies the version.

```yaml
---
# Cartouche v1
title: "Document Title"
author:
  name: "Author Name or Pseudonym"
  copyright: "© 2026 Benoit Pereira da Silva"
created: 2026-02-09
revised: 2026-02-09
lang: fr-FR
origin_lang: fr-FR
translation_of: null
translator: null
access:
  humans: true
  agents: false
status: review
---
```

---

## Fields

### `title` — Document Title

The canonical, human-readable title. Used for indexing and cross-referencing.

- **Required**: yes
- **Type**: string

### `author` — Authorship

Identifies who wrote the document and who holds copyright.

- **Required**: yes
- **Type**: object

| Sub-field   | Type   | Required | Description                              |
|-------------|--------|----------|------------------------------------------|
| `name`      | string | yes      | Author name or pseudonym                 |
| `copyright` | string | yes      | Copyright holder and year                |

### `created` — Creation Date

The date the document was first written.

- **Required**: yes
- **Type**: date (`YYYY-MM-DD`)

### `revised` — Last Revision Date

The date of the most recent substantive edit. Updated by the author
(human or agent) whenever content changes meaningfully.

- **Required**: yes
- **Type**: date (`YYYY-MM-DD`)

### `lang` — Document Language

The language this file is written in, as a BCP-47 tag.

- **Required**: yes
- **Type**: string (BCP-47, e.g. `fr-FR`, `en-US`)

### `origin_lang` — Original Language

The language in which the text was **originally conceived and written**.

- **Required**: yes
- **Type**: string (BCP-47)

**Key rule**: if `lang == origin_lang`, the document is in its original language.
If they differ, this document is a translation.

### `translation_of` — Source Document

If this document is a translation, the relative path to the original document.

- **Required**: no (use `null` if not a translation)
- **Type**: string (relative path) or `null`

### `translator` — Translator

If this document is a translation, the name of the translator.
Translation is an act of authorship; the translator deserves credit.
This field accepts human names, synthetic intelligence names, or both.

- **Required**: yes (use `null` if not a translation)
- **Type**: string or `null`

**Example**: a file `BONJOUR_en_US.md` translated from French would have:

```yaml
lang: en-US
origin_lang: fr-FR
translation_of: ./BONJOUR_fr_FR.md
translator: Claude
```

### `access` — Access Control

Declares who is allowed to read this document.

- **Required**: yes
- **Type**: object

| Sub-field | Type    | Required | Description                           |
|-----------|---------|----------|---------------------------------------|
| `humans`  | boolean | yes      | Whether human readers may access this |
| `agents`  | boolean | yes      | Whether AI agents may access this     |

**Future extension**: the `agents` field may evolve into an object with
finer-grained permissions (`read`, `index`, `execute`).

### `status` — Document Maturity

The lifecycle stage of the document.

- **Required**: yes
- **Type**: enum

| Value     | Meaning                                              |
|-----------|------------------------------------------------------|
| `draft`   | Work in progress, may change substantially           |
| `review`  | Content is complete, awaiting validation             |
| `stable`  | Approved and authoritative                           |
| `archived`| Superseded or no longer maintained                   |

---

## Conventions

1. **Every Markdown file** in an Organic Programming project should have a cartouche.
2. **Agents must parse the cartouche** before reading the document body.
   If `access.agents` is `false`, the agent must stop reading immediately.
3. **The `revised` date** must be updated on every substantive edit.
4. **File naming** should reflect the language: `DOCUMENT_<lang>.md`
   (e.g. `BONJOUR_fr_FR.md`, `BONJOUR_en_US.md`).
5. **The cartouche comment** (`# Cartouche v1`) enables future parsers to
   detect the schema version without external tooling.

---

## INDEX.md — The Directory Manifest

Every folder in an Organic Programming project should contain an `INDEX.md`
that lists all related documents with their key cartouche fields.

### Purpose

The index solves an efficiency problem: without it, an agent must open every
file and parse its cartouche to discover what it may read. The index provides
a **cached summary** of each document's access, language, and status — so that
navigation requires opening only one file.

### Structure

The `INDEX.md` itself carries a cartouche (with `access.agents: true` — it
must be readable by agents since it's their navigation entry point).

The body contains two Markdown tables:

1. **Documents in this folder** — files in the same directory.
2. **Related documents outside this folder** — cross-references to documents
   in other directories, enabling inter-folder navigation.

Each entry has five columns:

| Column | Content |
|---|---|
| **Document** | Markdown link to the file |
| **Title** | Cached from the cartouche `title` field |
| **Access** | Shorthand: `H` = humans, `A` = agents, `H · A` = both |
| **Lang** | Cached from the cartouche `lang` field |
| **Status** | Cached from the cartouche `status` field |

### The Access shorthand

The `Access` column uses a compact notation:

| Notation | Meaning |
|---|---|
| `H` | Humans only |
| `A` | Agents only |
| `H · A` | Both humans and agents |

This is a **cache** for fast lookup. The cartouche inside each file remains
the authoritative source of truth. If they disagree, the cartouche wins.

### Consistency rule

When a document's cartouche changes, the corresponding `INDEX.md` entry should
be updated to match. This is the responsibility of whoever (human or agent)
modifies the cartouche.

---

## File Naming Convention

Filenames encode **only the language**, not access control or status:

```
DOCUMENT_<lang>.md
```

Examples: `BONJOUR_fr_FR.md`, `DSE_en_US.md`, `CARTOUCHE.md` (language-neutral).

**Rationale**: language is a *routing* concern (a reader browsing a repo needs to
find the right file). Access control is a *policy* concern — it belongs in the
cartouche, cached in `INDEX.md`, never in the filename. This avoids DRY violations,
combinatorial explosion of naming segments, and costly renames when policies change.

---

## Relationship to AGENT.md

The cartouche complements Article 10 of the Organic Programming constitution.
While `AGENT.md` governs *how agents behave*, the cartouche governs
*what agents (and humans) are allowed to read*, document by document.
The `INDEX.md` serves as the navigational bridge — it is the first file an agent
reads upon entering a folder to discover the available documents and their access rights.
