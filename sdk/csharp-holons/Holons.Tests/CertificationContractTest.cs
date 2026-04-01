using System.Diagnostics;
using System.Text.Json.Nodes;

namespace Holons.Tests;

public class CertificationContractTest
{
    [Fact]
    public void EchoWrapperScriptsExistAndAreExecutable()
    {
        var root = ProjectRoot();
        var echoClient = Path.Combine(root, "bin", "echo-client");
        var echoServer = Path.Combine(root, "bin", "echo-server");
        var holonRpcServer = Path.Combine(root, "bin", "holon-rpc-server");

        Assert.True(File.Exists(echoClient), "echo-client script is missing");
        Assert.True(File.Exists(echoServer), "echo-server script is missing");
        Assert.True(File.Exists(holonRpcServer), "holon-rpc-server script is missing");

        var clientText = File.ReadAllText(echoClient);
        var serverText = File.ReadAllText(echoServer);
        var holonRpcServerText = File.ReadAllText(holonRpcServer);
        Assert.Contains("cmd/echo-client-go/main.go", clientText);
        Assert.Contains("cmd/echo-server-go/main.go", serverText);
        Assert.Contains("forward_signal", serverText);
        Assert.Contains("cmd/holon-rpc-server-go/main.go", holonRpcServerText);

        if (!OperatingSystem.IsWindows())
        {
            var clientMode = File.GetUnixFileMode(echoClient);
            var serverMode = File.GetUnixFileMode(echoServer);
            var holonRpcServerMode = File.GetUnixFileMode(holonRpcServer);
            Assert.True(clientMode.HasFlag(UnixFileMode.UserExecute), "echo-client is not executable");
            Assert.True(serverMode.HasFlag(UnixFileMode.UserExecute), "echo-server is not executable");
            Assert.True(holonRpcServerMode.HasFlag(UnixFileMode.UserExecute), "holon-rpc-server is not executable");
        }
    }

    [Fact]
    public async Task EchoClientWrapperInvokesGoHelperWithExpectedArguments()
    {
        if (OperatingSystem.IsWindows())
            return;

        var root = ProjectRoot();
        var tmpDir = Directory.CreateTempSubdirectory("holons-csharp-echo-wrapper-");
        var logPath = Path.Combine(tmpDir.FullName, "fake-go.log");
        var fakeGo = Path.Combine(tmpDir.FullName, "fake-go.sh");

        await File.WriteAllTextAsync(fakeGo, """
            #!/usr/bin/env bash
            set -euo pipefail
            log_file="${FAKE_GO_LOG:?}"
            {
              printf 'CWD=%s\n' "$PWD"
              i=0
              for arg in "$@"; do
                printf 'ARG%d=%s\n' "$i" "$arg"
                i=$((i+1))
              done
            } > "$log_file"
            exit 0
            """);
        File.SetUnixFileMode(
            fakeGo,
            UnixFileMode.UserRead | UnixFileMode.UserWrite | UnixFileMode.UserExecute);

        var process = new Process
        {
            StartInfo = new ProcessStartInfo
            {
                FileName = Path.Combine(root, "bin", "echo-client"),
                WorkingDirectory = root,
                RedirectStandardOutput = true,
                RedirectStandardError = true,
                UseShellExecute = false
            }
        };
        process.StartInfo.ArgumentList.Add("stdio://");
        process.StartInfo.ArgumentList.Add("--message");
        process.StartInfo.ArgumentList.Add("cert-stdio");
        process.StartInfo.Environment["GO_BIN"] = fakeGo;
        process.StartInfo.Environment["FAKE_GO_LOG"] = logPath;

        process.Start();
        await process.WaitForExitAsync();

        Assert.Equal(0, process.ExitCode);
        Assert.True(File.Exists(logPath), "fake go invocation log missing");

        var log = await File.ReadAllTextAsync(logPath);
        Assert.Contains("CWD=", log);
        Assert.Contains("go-holons", log);
        Assert.Contains("ARG0=run", log);
        Assert.Contains("cmd/echo-client-go/main.go", log);
        Assert.Contains("--sdk", log);
        Assert.Contains("csharp-holons", log);
        Assert.Contains("--server-sdk", log);
        Assert.Contains("go-holons", log);
        Assert.Contains("stdio://", log);
        Assert.Contains("--message", log);
        Assert.Contains("cert-stdio", log);
    }

    [Fact]
    public async Task EchoServerWrapperPreservesServeArgumentOrder()
    {
        if (OperatingSystem.IsWindows())
            return;

        var root = ProjectRoot();
        var log = await RunWrapperWithFakeGo(
            Path.Combine(root, "bin", "echo-server"),
            "serve",
            "--listen",
            "stdio://");

        Assert.Contains("ARG0=run", log);
        Assert.Contains($"ARG1={Path.Combine(root, "cmd", "echo-server-go", "main.go")}", log);
        Assert.Contains("ARG2=serve", log);
        Assert.Contains("ARG3=--listen", log);
        Assert.Contains("ARG4=stdio://", log);
        Assert.Contains("ARG5=--sdk", log);
        Assert.Contains("ARG6=csharp-holons", log);
    }

    [Fact]
    public async Task EchoServerWrapperForwardsSigtermAndExitsZero()
    {
        if (OperatingSystem.IsWindows())
            return;

        var root = ProjectRoot();
        var tmpDir = Directory.CreateTempSubdirectory("holons-csharp-signal-wrapper-");
        var logPath = Path.Combine(tmpDir.FullName, "fake-go.log");
        var fakeGo = Path.Combine(tmpDir.FullName, "fake-go.sh");
        Process? process = null;

        try
        {
            await File.WriteAllTextAsync(fakeGo, """
                #!/usr/bin/env bash
                set -euo pipefail
                : "${FAKE_GO_LOG:?FAKE_GO_LOG is required}"
                printf 'CWD=%s\n' "${PWD}" > "${FAKE_GO_LOG}"
                index=0
                for arg in "$@"; do
                  printf 'ARG%d=%s\n' "${index}" "${arg}" >> "${FAKE_GO_LOG}"
                  index=$((index + 1))
                done

                child() {
                  trap 'printf "CHILD_TERM=1\n" >> "${FAKE_GO_LOG}"; exit 0' TERM INT
                  printf 'CHILD_READY=1\n' >> "${FAKE_GO_LOG}"
                  while true; do
                    sleep 0.1
                  done
                }

                if [[ "${1:-}" == "run" ]]; then
                  child &
                  child_pid=$!
                  printf 'GO_PID=%d\n' "$$" >> "${FAKE_GO_LOG}"
                  printf 'CHILD_PID=%d\n' "${child_pid}" >> "${FAKE_GO_LOG}"
                  wait "${child_pid}"
                fi
                """);
            File.SetUnixFileMode(
                fakeGo,
                UnixFileMode.UserRead | UnixFileMode.UserWrite | UnixFileMode.UserExecute);

            process = new Process
            {
                StartInfo = new ProcessStartInfo
                {
                    FileName = Path.Combine(root, "bin", "echo-server"),
                    WorkingDirectory = root,
                    RedirectStandardOutput = true,
                    RedirectStandardError = true,
                    UseShellExecute = false
                }
            };
            process.StartInfo.ArgumentList.Add("--listen");
            process.StartInfo.ArgumentList.Add("tcp://127.0.0.1:0");
            process.StartInfo.Environment["GO_BIN"] = fakeGo;
            process.StartInfo.Environment["FAKE_GO_LOG"] = logPath;

            process.Start();

            var ready = false;
            for (var i = 0; i < 40; i++)
            {
                if (File.Exists(logPath))
                {
                    var capture = await File.ReadAllTextAsync(logPath);
                    if (capture.Contains("CHILD_READY=1", StringComparison.Ordinal))
                    {
                        ready = true;
                        break;
                    }
                }
                await Task.Delay(50);
            }

            Assert.True(ready, "fake go child was not ready");

            using (var killer = new Process
            {
                StartInfo = new ProcessStartInfo
                {
                    FileName = "kill",
                    RedirectStandardOutput = true,
                    RedirectStandardError = true,
                    UseShellExecute = false
                }
            })
            {
                killer.StartInfo.ArgumentList.Add("-TERM");
                killer.StartInfo.ArgumentList.Add(process.Id.ToString());
                killer.Start();
                await killer.WaitForExitAsync();
                Assert.Equal(0, killer.ExitCode);
            }

            var exitTask = process.WaitForExitAsync();
            var completed = await Task.WhenAny(exitTask, Task.Delay(TimeSpan.FromSeconds(10)));
            Assert.True(completed == exitTask, "echo-server wrapper did not exit on SIGTERM");
            Assert.Equal(0, process.ExitCode);
        }
        finally
        {
            try
            {
                if (process is { HasExited: false })
                    process.Kill(entireProcessTree: true);
            }
            catch
            {
                // ignored
            }
            finally
            {
                process?.Dispose();
            }

            try
            {
                if (File.Exists(logPath))
                    File.Delete(logPath);
            }
            catch
            {
                // ignored
            }

            tmpDir.Delete(recursive: true);
        }
    }

    [Fact]
    public async Task HolonRpcServerWrapperForwardsSdkAndUri()
    {
        if (OperatingSystem.IsWindows())
            return;

        var root = ProjectRoot();
        var log = await RunWrapperWithFakeGo(
            Path.Combine(root, "bin", "holon-rpc-server"),
            "ws://127.0.0.1:8080/rpc",
            "--once");

        Assert.Contains("ARG0=run", log);
        Assert.Contains($"ARG1={Path.Combine(root, "cmd", "holon-rpc-server-go", "main.go")}", log);
        Assert.Contains("ARG2=--sdk", log);
        Assert.Contains("ARG3=csharp-holons", log);
        Assert.Contains("ARG4=ws://127.0.0.1:8080/rpc", log);
        Assert.Contains("ARG5=--once", log);
    }

    [Fact]
    public async Task EchoClientWrapperSupportsWsDial()
    {
        if (OperatingSystem.IsWindows())
            return;

        var root = ProjectRoot();
        var goHolonsDir = Path.Combine(root, "..", "go-holons");

        using var goServer = new Process
        {
            StartInfo = new ProcessStartInfo
            {
                FileName = ResolveGoBinary(),
                WorkingDirectory = goHolonsDir,
                RedirectStandardOutput = true,
                RedirectStandardError = true,
                UseShellExecute = false
            }
        };
        goServer.StartInfo.ArgumentList.Add("run");
        goServer.StartInfo.ArgumentList.Add("./cmd/echo-server");
        goServer.StartInfo.ArgumentList.Add("--listen");
        goServer.StartInfo.ArgumentList.Add("ws://127.0.0.1:0/grpc");
        goServer.StartInfo.ArgumentList.Add("--sdk");
        goServer.StartInfo.ArgumentList.Add("go-holons");

        goServer.Start();
        var wsURI = await ReadLineWithTimeout(goServer.StandardOutput, TimeSpan.FromSeconds(20));
        if (string.IsNullOrWhiteSpace(wsURI))
        {
            var stderr = await goServer.StandardError.ReadToEndAsync();
            throw new InvalidOperationException($"go ws echo-server failed to start: {stderr}");
        }

        try
        {
            using var client = new Process
            {
                StartInfo = new ProcessStartInfo
                {
                    FileName = Path.Combine(root, "bin", "echo-client"),
                    WorkingDirectory = root,
                    RedirectStandardOutput = true,
                    RedirectStandardError = true,
                    UseShellExecute = false
                }
            };
            client.StartInfo.ArgumentList.Add("--message");
            client.StartInfo.ArgumentList.Add("cert-ws");
            client.StartInfo.ArgumentList.Add("--server-sdk");
            client.StartInfo.ArgumentList.Add("go-holons");
            client.StartInfo.ArgumentList.Add(wsURI);

            client.Start();
            await client.WaitForExitAsync();

            var output = await client.StandardOutput.ReadToEndAsync();
            var error = await client.StandardError.ReadToEndAsync();

            Assert.Equal(0, client.ExitCode);
            Assert.Contains("\"status\":\"pass\"", output);
            Assert.Contains("\"response_sdk\":\"go-holons\"", output);
            Assert.True(string.IsNullOrWhiteSpace(error) || error.Contains("serve failed: EOF", StringComparison.Ordinal));
        }
        finally
        {
            try
            {
                if (!goServer.HasExited)
                    goServer.Kill(entireProcessTree: true);
            }
            catch
            {
                // ignored
            }

            await goServer.WaitForExitAsync();
        }
    }

    [Fact]
    public async Task HolonRpcServerWrapperServesEcho()
    {
        if (OperatingSystem.IsWindows())
            return;

        var root = ProjectRoot();
        using var server = new Process
        {
            StartInfo = new ProcessStartInfo
            {
                FileName = Path.Combine(root, "bin", "holon-rpc-server"),
                WorkingDirectory = root,
                RedirectStandardOutput = true,
                RedirectStandardError = true,
                UseShellExecute = false
            }
        };
        server.StartInfo.ArgumentList.Add("--once");

        server.Start();

        try
        {
            var url = await ReadLineWithTimeout(server.StandardOutput, TimeSpan.FromSeconds(20));
            if (string.IsNullOrWhiteSpace(url))
            {
                var stderr = await server.StandardError.ReadToEndAsync();
                throw new InvalidOperationException($"holon-rpc-server wrapper failed to start: {stderr}");
            }

            await using var client = new HolonRPCClient(
                heartbeatIntervalMs: 250,
                heartbeatTimeoutMs: 250,
                reconnectMinDelayMs: 100,
                reconnectMaxDelayMs: 400);

            await client.ConnectAsync(url);
            var result = await client.InvokeAsync(
                "echo.v1.Echo/Ping",
                new JsonObject { ["message"] = "cert-holonrpc" });

            Assert.Equal("cert-holonrpc", result["message"]?.GetValue<string>());
            Assert.Equal("csharp-holons", result["sdk"]?.GetValue<string>());
            await client.CloseAsync();

            var exitTask = server.WaitForExitAsync();
            var exited = await Task.WhenAny(exitTask, Task.Delay(TimeSpan.FromSeconds(10)));
            Assert.True(exited == exitTask, "holon-rpc-server wrapper did not exit in once mode");
            Assert.Equal(0, server.ExitCode);
        }
        finally
        {
            try
            {
                if (!server.HasExited)
                    server.Kill(entireProcessTree: true);
            }
            catch
            {
                // ignored
            }

            await server.WaitForExitAsync();
        }
    }

    [Fact]
    public async Task EchoServerWrapperPropagatesDeadlineWithHandlerDelay()
    {
        if (OperatingSystem.IsWindows())
            return;

        var root = ProjectRoot();
        var goHolonsDir = Path.Combine(root, "..", "go-holons");

        using var server = new Process
        {
            StartInfo = new ProcessStartInfo
            {
                FileName = Path.Combine(root, "bin", "echo-server"),
                WorkingDirectory = root,
                RedirectStandardOutput = true,
                RedirectStandardError = true,
                UseShellExecute = false
            }
        };
        server.StartInfo.ArgumentList.Add("--listen");
        server.StartInfo.ArgumentList.Add("tcp://127.0.0.1:0");
        server.StartInfo.ArgumentList.Add("--handler-delay-ms");
        server.StartInfo.ArgumentList.Add("5000");

        server.Start();

        try
        {
            var uri = await ReadLineWithTimeout(server.StandardOutput, TimeSpan.FromSeconds(20));
            if (string.IsNullOrWhiteSpace(uri))
            {
                var stderr = await server.StandardError.ReadToEndAsync();
                throw new InvalidOperationException($"echo-server failed to start in delay mode: {stderr}");
            }

            using var timeoutClient = new Process
            {
                StartInfo = new ProcessStartInfo
                {
                    FileName = ResolveGoBinary(),
                    WorkingDirectory = goHolonsDir,
                    RedirectStandardOutput = true,
                    RedirectStandardError = true,
                    UseShellExecute = false
                }
            };
            timeoutClient.StartInfo.ArgumentList.Add("run");
            timeoutClient.StartInfo.ArgumentList.Add("./cmd/echo-client");
            timeoutClient.StartInfo.ArgumentList.Add("--server-sdk");
            timeoutClient.StartInfo.ArgumentList.Add("csharp-holons");
            timeoutClient.StartInfo.ArgumentList.Add("--message");
            timeoutClient.StartInfo.ArgumentList.Add("timeout-check");
            timeoutClient.StartInfo.ArgumentList.Add("--timeout-ms");
            timeoutClient.StartInfo.ArgumentList.Add("2000");
            timeoutClient.StartInfo.ArgumentList.Add(uri);

            timeoutClient.Start();
            await timeoutClient.WaitForExitAsync();

            var timeoutOut = await timeoutClient.StandardOutput.ReadToEndAsync();
            var timeoutErr = await timeoutClient.StandardError.ReadToEndAsync();
            Assert.NotEqual(0, timeoutClient.ExitCode);
            Assert.True(
                timeoutErr.Contains("DeadlineExceeded", StringComparison.Ordinal) ||
                timeoutErr.Contains("deadline exceeded", StringComparison.OrdinalIgnoreCase),
                $"expected deadline exceeded, stdout={timeoutOut}\nstderr={timeoutErr}");

            using var followupClient = new Process
            {
                StartInfo = new ProcessStartInfo
                {
                    FileName = ResolveGoBinary(),
                    WorkingDirectory = goHolonsDir,
                    RedirectStandardOutput = true,
                    RedirectStandardError = true,
                    UseShellExecute = false
                }
            };
            followupClient.StartInfo.ArgumentList.Add("run");
            followupClient.StartInfo.ArgumentList.Add("./cmd/echo-client");
            followupClient.StartInfo.ArgumentList.Add("--server-sdk");
            followupClient.StartInfo.ArgumentList.Add("csharp-holons");
            followupClient.StartInfo.ArgumentList.Add("--message");
            followupClient.StartInfo.ArgumentList.Add("timeout-followup");
            followupClient.StartInfo.ArgumentList.Add("--timeout-ms");
            followupClient.StartInfo.ArgumentList.Add("7000");
            followupClient.StartInfo.ArgumentList.Add(uri);

            followupClient.Start();
            await followupClient.WaitForExitAsync();

            var followupOut = await followupClient.StandardOutput.ReadToEndAsync();
            var followupErr = await followupClient.StandardError.ReadToEndAsync();
            Assert.Equal(0, followupClient.ExitCode);
            Assert.Contains("\"status\":\"pass\"", followupOut);
            Assert.True(string.IsNullOrWhiteSpace(followupErr), followupErr);
        }
        finally
        {
            try
            {
                if (!server.HasExited)
                    server.Kill(entireProcessTree: true);
            }
            catch
            {
                // ignored
            }

            await server.WaitForExitAsync();
        }
    }

    [Fact]
    public async Task EchoServerWrapperRejectsOversizedMessagesAndStaysHealthy()
    {
        if (OperatingSystem.IsWindows())
            return;

        var root = ProjectRoot();
        var goHolonsDir = Path.Combine(root, "..", "go-holons");
        var probeFile = Path.Combine(Path.GetTempPath(), $"csharp-holons-l5-7-{Guid.NewGuid():N}.go");
        await File.WriteAllTextAsync(probeFile, GoOversizeProbeSource);

        using var server = new Process
        {
            StartInfo = new ProcessStartInfo
            {
                FileName = Path.Combine(root, "bin", "echo-server"),
                WorkingDirectory = root,
                RedirectStandardOutput = true,
                RedirectStandardError = true,
                UseShellExecute = false
            }
        };
        server.StartInfo.ArgumentList.Add("--listen");
        server.StartInfo.ArgumentList.Add("tcp://127.0.0.1:0");

        server.Start();

        try
        {
            var uri = await ReadLineWithTimeout(server.StandardOutput, TimeSpan.FromSeconds(20));
            if (string.IsNullOrWhiteSpace(uri))
            {
                var stderr = await server.StandardError.ReadToEndAsync();
                throw new InvalidOperationException($"echo-server failed to start for oversize probe: {stderr}");
            }

            using var probe = new Process
            {
                StartInfo = new ProcessStartInfo
                {
                    FileName = ResolveGoBinary(),
                    WorkingDirectory = goHolonsDir,
                    RedirectStandardOutput = true,
                    RedirectStandardError = true,
                    UseShellExecute = false
                }
            };
            probe.StartInfo.ArgumentList.Add("run");
            probe.StartInfo.ArgumentList.Add(probeFile);
            probe.StartInfo.ArgumentList.Add(uri);

            probe.Start();
            await probe.WaitForExitAsync();

            var probeOut = await probe.StandardOutput.ReadToEndAsync();
            var probeErr = await probe.StandardError.ReadToEndAsync();
            Assert.Equal(0, probe.ExitCode);
            Assert.Contains("RESULT=RESOURCE_EXHAUSTED", probeOut);
            Assert.Contains("SMALL=OK", probeOut);
            Assert.True(string.IsNullOrWhiteSpace(probeErr), probeErr);
        }
        finally
        {
            try
            {
                if (File.Exists(probeFile))
                    File.Delete(probeFile);
            }
            catch
            {
                // ignored
            }

            try
            {
                if (!server.HasExited)
                    server.Kill(entireProcessTree: true);
            }
            catch
            {
                // ignored
            }

            await server.WaitForExitAsync();
        }
    }

    private static async Task<string> RunWrapperWithFakeGo(string wrapperPath, params string[] args)
    {
        var root = ProjectRoot();
        var tmpDir = Directory.CreateTempSubdirectory("holons-csharp-wrapper-");
        var logPath = Path.Combine(tmpDir.FullName, "fake-go.log");
        var fakeGo = Path.Combine(tmpDir.FullName, "fake-go.sh");

        try
        {
            await File.WriteAllTextAsync(fakeGo, """
                #!/usr/bin/env bash
                set -euo pipefail
                log_file="${FAKE_GO_LOG:?}"
                {
                  printf 'CWD=%s\n' "$PWD"
                  i=0
                  for arg in "$@"; do
                    printf 'ARG%d=%s\n' "$i" "$arg"
                    i=$((i+1))
                  done
                } > "$log_file"
                exit 0
                """);
            if (!OperatingSystem.IsWindows())
            {
                File.SetUnixFileMode(
                    fakeGo,
                    UnixFileMode.UserRead | UnixFileMode.UserWrite | UnixFileMode.UserExecute);
            }

            var process = new Process
            {
                StartInfo = new ProcessStartInfo
                {
                    FileName = wrapperPath,
                    WorkingDirectory = root,
                    RedirectStandardOutput = true,
                    RedirectStandardError = true,
                    UseShellExecute = false
                }
            };
            foreach (var arg in args)
                process.StartInfo.ArgumentList.Add(arg);
            process.StartInfo.Environment["GO_BIN"] = fakeGo;
            process.StartInfo.Environment["FAKE_GO_LOG"] = logPath;

            process.Start();
            await process.WaitForExitAsync();

            Assert.Equal(0, process.ExitCode);
            Assert.True(File.Exists(logPath), "fake go invocation log missing");

            return await File.ReadAllTextAsync(logPath);
        }
        finally
        {
            tmpDir.Delete(recursive: true);
        }
    }

    private static async Task<string?> ReadLineWithTimeout(StreamReader reader, TimeSpan timeout)
    {
        var lineTask = reader.ReadLineAsync();
        var completed = await Task.WhenAny(lineTask, Task.Delay(timeout));
        if (completed != lineTask)
            return null;
        return await lineTask;
    }

    private static string ResolveGoBinary()
    {
        var preferred = "/Users/bpds/go/go1.25.1/bin/go";
        return File.Exists(preferred) ? preferred : "go";
    }

    private const string GoOversizeProbeSource = """
        package main

        import (
          "context"
          "encoding/json"
          "fmt"
          "os"
          "strings"
          "time"

          "google.golang.org/grpc"
          "google.golang.org/grpc/credentials/insecure"
        )

        type PingRequest struct { Message string `json:"message"` }
        type PingResponse struct { Message string `json:"message"`; SDK string `json:"sdk"` }
        type jsonCodec struct{}

        func (jsonCodec) Name() string { return "json" }
        func (jsonCodec) Marshal(v any) ([]byte, error) { return json.Marshal(v) }
        func (jsonCodec) Unmarshal(data []byte, v any) error { return json.Unmarshal(data, v) }

        func main() {
          if len(os.Args) != 2 {
            fmt.Println("RESULT=BAD_ARGS")
            os.Exit(2)
          }

          target := strings.TrimPrefix(os.Args[1], "tcp://")
          dialCtx, cancelDial := context.WithTimeout(context.Background(), 5*time.Second)
          defer cancelDial()

          conn, err := grpc.DialContext(dialCtx, target,
            grpc.WithTransportCredentials(insecure.NewCredentials()),
            grpc.WithBlock(),
            grpc.WithDefaultCallOptions(grpc.ForceCodec(jsonCodec{})),
          )
          if err != nil {
            fmt.Printf("RESULT=DIAL_ERROR err=%v\n", err)
            os.Exit(1)
          }
          defer conn.Close()

          big := strings.Repeat("x", 2*1024*1024)
          reqCtx, cancelReq := context.WithTimeout(context.Background(), 5*time.Second)
          defer cancelReq()

          var largeOut PingResponse
          err = conn.Invoke(reqCtx, "/echo.v1.Echo/Ping", &PingRequest{Message: big}, &largeOut, grpc.ForceCodec(jsonCodec{}))
          if err != nil {
            low := strings.ToLower(err.Error())
            if strings.Contains(low, "resource_exhausted") || strings.Contains(low, "resourceexhausted") {
              fmt.Println("RESULT=RESOURCE_EXHAUSTED")
            } else {
              fmt.Printf("RESULT=ERROR err=%v\n", err)
            }
          } else {
            fmt.Printf("RESULT=OK RESP_LEN=%d SDK=%s\n", len(largeOut.Message), largeOut.SDK)
          }

          var smallOut PingResponse
          err = conn.Invoke(context.Background(), "/echo.v1.Echo/Ping", &PingRequest{Message: "ok"}, &smallOut, grpc.ForceCodec(jsonCodec{}))
          if err != nil {
            fmt.Printf("SMALL=ERROR err=%v\n", err)
            return
          }

          fmt.Printf("SMALL=OK SDK=%s\n", smallOut.SDK)
        }
        """;

    private static string ProjectRoot()
    {
        return TestPaths.CSharpHolonsRoot();
    }
}
