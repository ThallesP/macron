/* What macOS itself takes to hand a mouse movement to an app: the native
 * side of tools/perf/inputlag.c, to tell Wine's share from the system's.
 *
 * Usage: mouselag <seconds> [coalesce]
 *
 * Opens a window at the same place as inputlag.exe's, and for every
 * mouse-moved event its app receives takes the time since the event's own
 * time stamp. Mouse coalescing is off, as Wine's Mac driver has it, unless
 * the second argument is `coalesce`. Prints RESULT lines like inputlag's.
 *
 * Build: clang -O2 -fobjc-arc -o mouselag mouselag.m -framework Cocoa */
#import <Cocoa/Cocoa.h>
#include <stdlib.h>

#define MAX_EVENTS 400000

static double *lag;
static size_t count;
static NSTimeInterval measure_from, measure_until;

@interface LagApplication : NSApplication
@end

@implementation LagApplication
- (void)sendEvent:(NSEvent *)event
{
    NSTimeInterval now = [[NSProcessInfo processInfo] systemUptime];
    if ([event type] == NSEventTypeMouseMoved && now >= measure_from && now < measure_until && count < MAX_EVENTS)
        lag[count++] = (now - [event timestamp]) * 1000.0;
    [super sendEvent:event];
}
@end

static int compare_doubles(const void *a, const void *b)
{
    double x = *(const double *)a, y = *(const double *)b;
    return (x > y) - (x < y);
}

int main(int argc, char **argv)
{
    double seconds = argc > 1 ? atof(argv[1]) : 15;
    BOOL coalesce = argc > 2 && !strcmp(argv[2], "coalesce");

    @autoreleasepool {
        lag = malloc(MAX_EVENTS * sizeof(*lag));
        NSApplication *app = [LagApplication sharedApplication];
        [app setActivationPolicy:NSApplicationActivationPolicyRegular];
        [NSEvent setMouseCoalescingEnabled:coalesce];

        NSRect screen = [[NSScreen mainScreen] frame];
        NSRect frame = NSMakeRect(100, screen.size.height - 100 - 600, 800, 600);
        NSWindow *window = [[NSWindow alloc] initWithContentRect:frame
                                                       styleMask:NSWindowStyleMaskBorderless
                                                         backing:NSBackingStoreBuffered
                                                           defer:NO];
        [window setTitle:@"Macron mouse lag"];
        [window setLevel:NSFloatingWindowLevel];
        [window setAcceptsMouseMovedEvents:YES];
        [window setBackgroundColor:[NSColor darkGrayColor]];
        [window makeKeyAndOrderFront:nil];
        [app activateIgnoringOtherApps:YES];

        measure_from = [[NSProcessInfo processInfo] systemUptime] + 4;
        measure_until = measure_from + seconds;
        [NSTimer scheduledTimerWithTimeInterval:seconds + 4.5
                                        repeats:NO
                                          block:^(NSTimer *timer) {
                                              [app stop:nil];
                                              [app postEvent:[NSEvent otherEventWithType:NSEventTypeApplicationDefined
                                                                                location:NSZeroPoint
                                                                           modifierFlags:0
                                                                               timestamp:0
                                                                            windowNumber:0
                                                                                 context:nil
                                                                                 subtype:0
                                                                                   data1:0
                                                                                   data2:0]
                                             atStart:YES];
                                          }];
        [app run];
    }

    printf("RESULT config seconds %.0f coalescing %s\n", seconds, coalesce ? "on" : "off");
    if (!count) {
        printf("RESULT mouse move: none received\n");
        return 1;
    }
    qsort(lag, count, sizeof(*lag), compare_doubles);
    size_t over_2 = 0, over_8 = 0, over_16 = 0;
    for (size_t i = 0; i < count; i++) {
        if (lag[i] > 2) over_2++;
        if (lag[i] > 8) over_8++;
        if (lag[i] > 16) over_16++;
    }
    printf("RESULT mouse move: %zu in %.1f s (%.0f a second), ms since the event p50 %.2f p95 %.2f p99 %.2f max %.2f min %.2f, over 2 ms %zu, over 8 ms %zu, over 16 ms %zu\n",
           count, seconds, count / seconds, lag[count / 2], lag[count * 95 / 100], lag[count * 99 / 100], lag[count - 1], lag[0], over_2,
           over_8, over_16);
    return 0;
}
