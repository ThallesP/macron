/* Moves the mouse at a steady rate, for tools/perf/inputlag.c on a sandbox.
 * Never run it on a Mac somebody is using: it moves their pointer.
 *
 * Usage: mousegen <seconds> <moves a second> <x> <y>
 *
 * Posts mouse-moved events that step the pointer two points right and two
 * points left of (x, y), in screen points from the top left, with the
 * matching deltas, as a mouse does. Prints how many it posted.
 *
 * Build: clang -O2 -o mousegen mousegen.c -framework ApplicationServices */
#include <ApplicationServices/ApplicationServices.h>
#include <mach/mach_time.h>
#include <stdio.h>
#include <stdlib.h>

int main(int argc, char **argv)
{
    if (argc != 5) {
        fprintf(stderr, "usage: mousegen <seconds> <moves a second> <x> <y>\n");
        return 2;
    }
    double seconds = atof(argv[1]), rate = atof(argv[2]), x = atof(argv[3]), y = atof(argv[4]);
    mach_timebase_info_data_t timebase;
    mach_timebase_info(&timebase);
    uint64_t step = (uint64_t)(1e9 / rate) * timebase.denom / timebase.numer;
    uint64_t next = mach_absolute_time();
    long total = (long)(seconds * rate), posted = 0;
    CGEventSourceRef source = CGEventSourceCreate(kCGEventSourceStateHIDSystemState);

    for (long i = 0; i < total; i++) {
        int delta = i % 2 ? -4 : 4;
        CGEventRef event = CGEventCreateMouseEvent(source, kCGEventMouseMoved, CGPointMake(x + delta / 2, y), kCGMouseButtonLeft);
        if (!event) break;
        CGEventSetIntegerValueField(event, kCGMouseEventDeltaX, delta);
        CGEventSetIntegerValueField(event, kCGMouseEventDeltaY, 0);
        CGEventPost(kCGHIDEventTap, event);
        CFRelease(event);
        posted++;
        next += step;
        mach_wait_until(next);
    }
    printf("posted %ld moves in %.1f s\n", posted, seconds);
    return 0;
}
