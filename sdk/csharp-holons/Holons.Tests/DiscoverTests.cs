using System;
using System.IO;

namespace Holons.Tests;

public class DiscoverTests
{
    [Fact]
    public void DiscoverRecursesSkipsAndDedups()
    {
        var root = Path.Combine(Path.GetTempPath(), $"holons-csharp-discover-{Guid.NewGuid():N}");
        Directory.CreateDirectory(root);
        try
        {
            WriteHolon(root, "holons/alpha", "uuid-alpha", "Alpha", "Go", "alpha-go");
            WriteHolon(root, "nested/beta", "uuid-beta", "Beta", "Rust", "beta-rust");
            WriteHolon(root, "nested/dup/alpha", "uuid-alpha", "Alpha", "Go", "alpha-go");

            foreach (var skipped in new[] { ".git/hidden", ".op/hidden", "node_modules/hidden", "vendor/hidden", "build/hidden", ".cache/hidden" })
                WriteHolon(root, skipped, $"ignored-{Path.GetFileName(skipped)}", "Ignored", "Holon", "ignored-holon");

            var entries = Discover.DiscoverRoot(root);
            Assert.Equal(2, entries.Count);

            var alpha = Assert.Single(entries.FindAll(entry => entry.Uuid == "uuid-alpha"));
            Assert.Equal("alpha-go", alpha.Slug);
            Assert.Equal("holons/alpha", alpha.RelativePath);
            Assert.Equal("go-module", alpha.Manifest?.Build.Runner);

            var beta = Assert.Single(entries.FindAll(entry => entry.Uuid == "uuid-beta"));
            Assert.Equal("nested/beta", beta.RelativePath);
        }
        finally
        {
            if (Directory.Exists(root))
                Directory.Delete(root, recursive: true);
        }
    }

    [Fact]
    public void DiscoverLocalAndFindHelpersUseCurrentDirectory()
    {
        var root = Path.Combine(Path.GetTempPath(), $"holons-csharp-find-{Guid.NewGuid():N}");
        Directory.CreateDirectory(root);

        var originalDir = Directory.GetCurrentDirectory();
        var originalOpPath = Environment.GetEnvironmentVariable("OPPATH");
        var originalOpBin = Environment.GetEnvironmentVariable("OPBIN");
        try
        {
            WriteHolon(root, "rob-go", "c7f3a1b2-1111-1111-1111-111111111111", "Rob", "Go", "rob-go");
            Directory.SetCurrentDirectory(root);
            Environment.SetEnvironmentVariable("OPPATH", Path.Combine(root, "runtime"));
            Environment.SetEnvironmentVariable("OPBIN", Path.Combine(root, "runtime", "bin"));

            var local = Discover.DiscoverLocal();
            Assert.Single(local);
            Assert.Equal("rob-go", local[0].Slug);

            var bySlug = Discover.FindBySlug("rob-go");
            Assert.NotNull(bySlug);
            Assert.Equal("c7f3a1b2-1111-1111-1111-111111111111", bySlug!.Uuid);

            var byUuid = Discover.FindByUUID("c7f3a1b2");
            Assert.NotNull(byUuid);
            Assert.Equal("rob-go", byUuid!.Slug);

            Assert.Null(Discover.FindBySlug("missing"));
        }
        finally
        {
            Directory.SetCurrentDirectory(originalDir);
            Environment.SetEnvironmentVariable("OPPATH", originalOpPath);
            Environment.SetEnvironmentVariable("OPBIN", originalOpBin);
            if (Directory.Exists(root))
                Directory.Delete(root, recursive: true);
        }
    }

    private static void WriteHolon(string root, string relativeDir, string uuid, string givenName, string familyName, string binary)
    {
        var dir = Path.Combine(root, relativeDir.Replace('/', Path.DirectorySeparatorChar));
        Directory.CreateDirectory(dir);
        File.WriteAllText(Path.Combine(dir, "holon.proto"), $$"""
            syntax = "proto3";
            package holons.test.v1;

            option (holons.v1.manifest) = {
              identity: {
                uuid: "{{uuid}}"
                given_name: "{{givenName}}"
                family_name: "{{familyName}}"
                motto: "Test"
                composer: "test"
                clade: "deterministic/pure"
                status: "draft"
                born: "2026-03-07"
              }
              lineage: {
                generated_by: "test"
              }
              kind: "native"
              build: {
                runner: "go-module"
              }
              artifacts: {
                binary: "{{binary}}"
              }
            };
            """);
    }
}
