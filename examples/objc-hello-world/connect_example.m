#import <Foundation/Foundation.h>
#import <Holons/Holons.h>

#include <signal.h>
#include <sys/stat.h>
#include <unistd.h>

static NSString *Trimmed(NSString *value) {
  return [value stringByTrimmingCharactersInSet:[NSCharacterSet whitespaceAndNewlineCharacterSet]];
}

static NSString *ProjectRoot(void) {
  return [[NSFileManager defaultManager] currentDirectoryPath];
}

static NSString *SDKBinary(NSString *name) {
  NSString *path = [[[ProjectRoot() stringByAppendingPathComponent:@"../../sdk/objc-holons/bin"]
      stringByAppendingPathComponent:name] stringByStandardizingPath];
  if (![[NSFileManager defaultManager] isExecutableFileAtPath:path]) {
    @throw [NSException exceptionWithName:NSInternalInconsistencyException
                                   reason:[NSString stringWithFormat:@"missing SDK helper: %@", path]
                                 userInfo:nil];
  }
  return path;
}

static void WriteFile(NSString *path, NSString *content) {
  NSError *error = nil;
  NSString *dir = [path stringByDeletingLastPathComponent];
  [[NSFileManager defaultManager] createDirectoryAtPath:dir
                            withIntermediateDirectories:YES
                                             attributes:nil
                                                  error:&error];
  if (error != nil ||
      ![content writeToFile:path atomically:YES encoding:NSUTF8StringEncoding error:&error]) {
    @throw [NSException exceptionWithName:NSInternalInconsistencyException
                                   reason:[NSString stringWithFormat:@"failed to write %@: %@",
                                                                       path, error.localizedDescription]
                                 userInfo:nil];
  }
}

static void WriteExecutable(NSString *path, NSString *content) {
  WriteFile(path, content);
  chmod(path.fileSystemRepresentation, 0755);
}

static void WriteEchoWrapperHolon(NSString *root,
                                  NSString *wrapperPath) {
  NSString *holonDir = [root stringByAppendingPathComponent:@"holons/echo-server"];
  [[NSFileManager defaultManager] createDirectoryAtPath:holonDir
                            withIntermediateDirectories:YES
                                             attributes:nil
                                                  error:nil];

  WriteFile(
      [holonDir stringByAppendingPathComponent:@"holon.yaml"],
      [NSString stringWithFormat:
                    @"uuid: \"echo-server-connect-example\"\n"
                     "given_name: Echo\n"
                     "family_name: Server\n"
                     "motto: Reply precisely.\n"
                     "composer: \"connect-example\"\n"
                     "kind: service\n"
                     "build:\n"
                     "  runner: objc\n"
                     "artifacts:\n"
                     "  binary: %@\n",
                    wrapperPath]);
}

static NSString *ReadTrimmedFileEventually(NSString *path, NSTimeInterval timeout) {
  NSDate *deadline = [NSDate dateWithTimeIntervalSinceNow:timeout];
  while ([deadline timeIntervalSinceNow] > 0) {
    NSError *error = nil;
    NSString *content = [NSString stringWithContentsOfFile:path
                                                  encoding:NSUTF8StringEncoding
                                                     error:&error];
    if (content != nil) {
      NSString *trimmed = Trimmed(content);
      if (trimmed.length > 0) {
        return trimmed;
      }
    }
    [NSThread sleepForTimeInterval:0.05];
  }
  return nil;
}

static BOOL WaitForPIDExit(pid_t pid, NSTimeInterval timeout) {
  NSDate *deadline = [NSDate dateWithTimeIntervalSinceNow:timeout];
  while ([deadline timeIntervalSinceNow] > 0) {
    if (kill(pid, 0) != 0) {
      return YES;
    }
    [NSThread sleepForTimeInterval:0.05];
  }
  return kill(pid, 0) != 0;
}

static NSString *DialTargetForGo(NSString *target) {
  NSString *trimmed = Trimmed(target);
  if ([trimmed hasPrefix:@"tcp://"]) {
    HOLParsedURI *parsed = HOLParseURI(trimmed);
    NSString *host = parsed.host ?: @"127.0.0.1";
    if (host.length == 0 || [host isEqualToString:@"0.0.0.0"] ||
        [host isEqualToString:@"::"] || [host isEqualToString:@"[::]"]) {
      host = @"127.0.0.1";
    }
    return [NSString stringWithFormat:@"%@:%d", host, parsed.port.intValue];
  }
  return trimmed;
}

static NSString *RunGoPingClient(NSString *root,
                                 NSString *target) {
  NSString *goDir = [root stringByAppendingPathComponent:@"go-client"];
  WriteFile(
      [goDir stringByAppendingPathComponent:@"go.mod"],
      @"module objcholonsconnect\n\n"
       "go 1.22.0\n\n"
       "require google.golang.org/grpc v1.78.0\n");
  WriteFile(
      [goDir stringByAppendingPathComponent:@"main.go"],
      @"package main\n"
       "import (\n"
       "  \"context\"\n"
       "  \"encoding/json\"\n"
       "  \"fmt\"\n"
       "  \"os\"\n"
       "  \"time\"\n"
       "  \"google.golang.org/grpc\"\n"
       "  \"google.golang.org/grpc/credentials/insecure\"\n"
       ")\n"
       "type PingRequest struct { Message string `json:\"message\"` }\n"
       "type PingResponse struct { Message string `json:\"message\"`; SDK string `json:\"sdk\"`; Version string `json:\"version\"` }\n"
       "type jsonCodec struct{}\n"
       "func (jsonCodec) Name() string { return \"json\" }\n"
       "func (jsonCodec) Marshal(v any) ([]byte, error) { return json.Marshal(v) }\n"
       "func (jsonCodec) Unmarshal(data []byte, v any) error { return json.Unmarshal(data, v) }\n"
       "func main() {\n"
       "  if len(os.Args) != 2 {\n"
       "    fmt.Fprintln(os.Stderr, \"target is required\")\n"
       "    os.Exit(2)\n"
       "  }\n"
       "  ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)\n"
       "  defer cancel()\n"
       "  conn, err := grpc.DialContext(ctx, os.Args[1], grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock(), grpc.WithDefaultCallOptions(grpc.ForceCodec(jsonCodec{})))\n"
       "  if err != nil {\n"
       "    fmt.Fprintln(os.Stderr, err)\n"
       "    os.Exit(1)\n"
       "  }\n"
       "  defer conn.Close()\n"
       "  var out PingResponse\n"
       "  if err := conn.Invoke(ctx, \"/echo.v1.Echo/Ping\", &PingRequest{Message: \"hello-from-objc\"}, &out); err != nil {\n"
       "    fmt.Fprintln(os.Stderr, err)\n"
       "    os.Exit(1)\n"
       "  }\n"
       "  if err := json.NewEncoder(os.Stdout).Encode(out); err != nil {\n"
       "    fmt.Fprintln(os.Stderr, err)\n"
       "    os.Exit(1)\n"
       "  }\n"
       "}\n");

  NSTask *task = [NSTask new];
  task.launchPath = @"/usr/bin/env";
  task.arguments = @[ @"go", @"run", @"-mod=mod", @".", DialTargetForGo(target) ];
  task.currentDirectoryPath = goDir;
  task.standardOutput = [NSPipe pipe];
  task.standardError = [NSPipe pipe];

  [task launch];
  [task waitUntilExit];

  NSData *stdoutData = [((NSPipe *)task.standardOutput).fileHandleForReading readDataToEndOfFile];
  NSData *stderrData = [((NSPipe *)task.standardError).fileHandleForReading readDataToEndOfFile];
  NSString *stdoutText = [[NSString alloc] initWithData:stdoutData encoding:NSUTF8StringEncoding] ?: @"";
  NSString *stderrText = [[NSString alloc] initWithData:stderrData encoding:NSUTF8StringEncoding] ?: @"";
  if (task.terminationStatus != 0) {
    @throw [NSException exceptionWithName:NSInternalInconsistencyException
                                   reason:[NSString stringWithFormat:@"go ping failed: %@",
                                                                       Trimmed(stderrText)]
                                 userInfo:nil];
  }
  return Trimmed(stdoutText);
}

int main(int argc, const char *argv[]) {
  @autoreleasepool {
    (void)argc;
    (void)argv;

    NSString *root = [NSTemporaryDirectory() stringByAppendingPathComponent:@"objc-holons-connect-example"];
    NSString *pidFile = [root stringByAppendingPathComponent:@"echo-wrapper.pid"];
    NSString *wrapperPath = [root stringByAppendingPathComponent:@"echo-wrapper.sh"];
    NSString *echoServer = SDKBinary(@"echo-server");
    NSString *previousCwd = [[NSFileManager defaultManager] currentDirectoryPath];
    GRPCChannel *channel = nil;

    @try {
      [[NSFileManager defaultManager] removeItemAtPath:root error:nil];
      [[NSFileManager defaultManager] createDirectoryAtPath:root
                                withIntermediateDirectories:YES
                                                 attributes:nil
                                                      error:nil];

      WriteExecutable(
          wrapperPath,
          [NSString stringWithFormat:
                        @"#!/usr/bin/env bash\n"
                         "set -euo pipefail\n"
                         "PID_FILE=%@\n"
                         "SERVER=%@\n"
                         "child=''\n"
                         "cleanup() {\n"
                         "  rm -f \"$PID_FILE\"\n"
                         "  if [[ -n \"$child\" ]] && kill -0 \"$child\" >/dev/null 2>&1; then\n"
                         "    kill -TERM \"$child\" >/dev/null 2>&1 || true\n"
                         "    wait \"$child\" >/dev/null 2>&1 || true\n"
                         "  fi\n"
                         "}\n"
                         "printf '%%s\\n' \"$$\" > \"$PID_FILE\"\n"
                         "trap cleanup EXIT INT TERM\n"
                         "\"$SERVER\" \"$@\" &\n"
                         "child=$!\n"
                         "wait \"$child\"\n",
                        pidFile, echoServer]);

      WriteEchoWrapperHolon(root, wrapperPath);
      [[NSFileManager defaultManager] changeCurrentDirectoryPath:root];

      HolonsConnectOptions *options = [HolonsConnectOptions new];
      options.transport = @"tcp";
      options.start = YES;
      channel = [Holons connect:@"echo-server" options:options];
      if (channel == nil) {
        @throw [NSException exceptionWithName:NSInternalInconsistencyException
                                       reason:@"Holons connect returned nil"
                                     userInfo:nil];
      }

      NSString *response = RunGoPingClient(root, channel.target);
      printf("%s\n", response.UTF8String);

      [Holons disconnect:channel];
      channel = nil;

      NSString *pidText = ReadTrimmedFileEventually(pidFile, 2.0);
      if (pidText.length > 0) {
        pid_t pid = (pid_t)pidText.intValue;
        if (pid > 0) {
          kill(pid, SIGTERM);
          WaitForPIDExit(pid, 2.0);
        }
      }
    } @finally {
      [Holons disconnect:channel];
      [[NSFileManager defaultManager] changeCurrentDirectoryPath:previousCwd];
      [[NSFileManager defaultManager] removeItemAtPath:root error:nil];
    }

    return 0;
  }
}
