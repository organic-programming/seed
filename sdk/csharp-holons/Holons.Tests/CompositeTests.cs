namespace Holons.Tests;

public class CompositeTests
{
    [Fact]
    public void MemberResolvesExecutableRelativeToLauncher()
    {
        var root = Path.Combine(Path.GetTempPath(), $"holons-csharp-composite-{Guid.NewGuid():N}");
        try
        {
            var launcher = Path.Combine(root, "bin", "darwin_arm64", "parent");
            var memberDir = Path.Combine(Path.GetDirectoryName(launcher)!, "holons", "csharp-node");
            var memberName = OperatingSystem.IsWindows() ? "observability-cascade-csharp-node.exe" : "observability-cascade-csharp-node";
            var member = Path.Combine(memberDir, memberName);
            Directory.CreateDirectory(memberDir);
            File.WriteAllText(launcher, "#!/bin/sh\n");
            File.WriteAllText(member, "#!/bin/sh\n");
            if (!OperatingSystem.IsWindows())
            {
                File.SetUnixFileMode(launcher, UnixFileMode.UserRead | UnixFileMode.UserExecute);
                File.SetUnixFileMode(member, UnixFileMode.UserRead | UnixFileMode.UserExecute);
            }

            Assert.Equal(member, Composite.MemberFromExecutable(launcher, "csharp-node"));
        }
        finally
        {
            if (Directory.Exists(root))
                Directory.Delete(root, recursive: true);
        }
    }

    [Fact]
    public void MemberErrorsWhenMissing()
    {
        var root = Path.Combine(Path.GetTempPath(), $"holons-csharp-composite-{Guid.NewGuid():N}");
        try
        {
            var launcher = Path.Combine(root, "bin", "darwin_arm64", "parent");
            Directory.CreateDirectory(Path.GetDirectoryName(launcher)!);
            File.WriteAllText(launcher, "#!/bin/sh\n");

            Assert.Throws<DirectoryNotFoundException>(() => Composite.MemberFromExecutable(launcher, "csharp-node"));
        }
        finally
        {
            if (Directory.Exists(root))
                Directory.Delete(root, recursive: true);
        }
    }
}
