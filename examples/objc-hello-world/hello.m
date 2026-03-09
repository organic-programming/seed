#import <Foundation/Foundation.h>
#import <Holons/Holons.h>

/// Pure deterministic HelloService.
NSString *HOLGreet(NSString *name) {
  NSString *n = (name.length == 0) ? @"World" : name;
  return [NSString stringWithFormat:@"Hello, %@!", n];
}

#ifndef TEST_BUILD
int main(int argc, const char *argv[]) {
  @autoreleasepool {
    if (argc > 1 && strcmp(argv[1], "serve") == 0) {
      NSMutableArray<NSString *> *serveArgs = [NSMutableArray array];
      for (int i = 2; i < argc; i++) {
        [serveArgs addObject:[NSString stringWithUTF8String:argv[i]]];
      }

      NSString *listenURI = HOLParseFlags(serveArgs);
      fprintf(stderr, "objc-hello-world listening on %s\n", listenURI.UTF8String);
      printf("{\"message\":\"%s\"}\n", HOLGreet(@"").UTF8String);
      return 0;
    }

    NSString *name = argc > 1 ? [NSString stringWithUTF8String:argv[1]] : @"";
    printf("%s\n", HOLGreet(name).UTF8String);
    return 0;
  }
}
#endif
