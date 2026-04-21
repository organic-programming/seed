namespace Holons;

public static partial class Discovery
{
    public const int LOCAL = 0;
    public const int PROXY = 1;
    public const int DELEGATED = 2;

    public const int SIBLINGS = 0x01;
    public const int CWD = 0x02;
    public const int SOURCE = 0x04;
    public const int BUILT = 0x08;
    public const int INSTALLED = 0x10;
    public const int CACHED = 0x20;
    public const int ALL = 0x3F;

    public const int NO_LIMIT = 0;
    public const int NO_TIMEOUT = 0;
}

public sealed record IdentityInfo(
    string GivenName,
    string FamilyName,
    string Motto,
    IReadOnlyList<string> Aliases);

public sealed record HolonInfo(
    string Slug,
    string Uuid,
    IdentityInfo Identity,
    string Lang,
    string Runner,
    string Status,
    string Kind,
    string Transport,
    string Entrypoint,
    IReadOnlyList<string> Architectures,
    bool HasDist,
    bool HasSource);

public sealed record HolonRef(string Url, HolonInfo? Info, string? Error);

public sealed record DiscoverResult(IReadOnlyList<HolonRef> Found, string? Error);

public sealed record ResolveResult(HolonRef? Ref, string? Error);

public sealed record ConnectResult(object? Channel, string UID, HolonRef? Origin, string? Error);
