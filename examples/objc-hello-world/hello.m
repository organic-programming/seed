#import <Foundation/Foundation.h>

/// Pure deterministic HelloService.
NSString *HOLGreet(NSString *name) {
  NSString *n = (name.length == 0) ? @"World" : name;
  return [NSString stringWithFormat:@"Hello, %@!", n];
}

#ifndef TEST_BUILD
int main(int argc, const char *argv[]) {
  @autoreleasepool {
    NSString *name = argc > 1 ? [NSString stringWithUTF8String:argv[1]] : @"";
    NSLog(@"%@", HOLGreet(name));
    return 0;
  }
}
#endif
