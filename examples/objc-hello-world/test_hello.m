#import <Foundation/Foundation.h>

// Forward declaration of greet — defined in hello.m
extern NSString *HOLGreet(NSString *name);

int main(int argc, const char *argv[]) {
  @autoreleasepool {
    int passed = 0, failed = 0;

    NSString *r1 = HOLGreet(@"Alice");
    if ([r1 isEqualToString:@"Hello, Alice!"])
      passed++;
    else {
      failed++;
      NSLog(@"FAIL: greet Alice: %@", r1);
    }

    NSString *r2 = HOLGreet(@"");
    if ([r2 isEqualToString:@"Hello, World!"])
      passed++;
    else {
      failed++;
      NSLog(@"FAIL: greet empty: %@", r2);
    }

    NSLog(@"%d passed, %d failed", passed, failed);
    return failed > 0 ? 1 : 0;
  }
}
