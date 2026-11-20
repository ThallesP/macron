// Checks the heat-following frame limit of patch 0006 (src/dxmt/dxmt_thermal.hpp)
// by itself: scripted seconds first, then a model of a Mac without a fan.
//
//   c++ -std=c++20 -I <patched dxmt>/src/dxmt -o thermal_pacer thermal_pacer.cpp && ./thermal_pacer [trace]
//
// runtime/dxmt/build.sh builds and runs it after the patches are applied; a
// failed check fails the build. With `trace` it prints the model's run.
//
// The model is fitted to Deadlock's practice map on a MacBook Air M3
// (docs/perf/latency.md, "Heat"): a temperature that follows the frame rate
// with a time constant of 83 s, level 1 at 0.79 and level 2 at 1.0, where 40
// frames a second is the rate whose steady temperature is exactly 1.0 (70
// reaches level 2 after 65 s, 60 after 100 s, 45 after 181 s, 36 never), and
// a ceiling on the frame rate that macOS lowers while the temperature is
// over 1.0 and gives back under it.
#include "dxmt_thermal.hpp"

#include <cmath>
#include <cstdio>
#include <cstdlib>
#include <cstring>

using dxmt::ThermalPacer;

static int failures;

static void
check(bool ok, const char *what, double got) {
  if (ok)
    return;
  failures++;
  printf("FAIL: %s (got %.2f)\n", what, got);
}

static double
feed(ThermalPacer &pacer, unsigned level, double rate, unsigned seconds) {
  double limit = 0;
  for (unsigned i = 0; i < seconds; i++)
    limit = pacer.update(level, pacer.limit() > 0 ? std::min(rate, pacer.limit()) : rate);
  return limit;
}

static void
scripted() {
  ThermalPacer cool;
  check(feed(cool, 0, 144, 600) == 0, "no limit at level 0", cool.limit());
  check(feed(cool, 1, 144, 3600) == 0, "no limit at level 1 for an hour: a Mac with a fan lives there", cool.limit());

  ThermalPacer pacer;
  feed(pacer, 0, 70, 50);
  feed(pacer, 1, 70, 15);
  double first = feed(pacer, 2, 70, 1);
  check(first > 40 && first < 50, "level 2 at 70 gives a limit of about two thirds at once", first);
  double later = feed(pacer, 2, 70, 20);
  check(later < first && later > first * 0.85, "a step lower after 20 s more at level 2", later);
  double floor = feed(pacer, 2, 70, 600);
  check(floor == ThermalPacer::kFloor, "ten minutes at level 2 end at the floor and not under it", floor);

  ThermalPacer back;
  feed(back, 0, 70, 60);
  double cut = feed(back, 2, 70, 1);
  double held = feed(back, 1, 70, 19);
  check(held == cut, "level 1 holds the limit for 20 s", held);
  double raised = feed(back, 1, 70, 280);
  check(raised > cut && raised <= 70 * 0.9, "level 1 raises it, not up to the rate that was too hot", raised);
  double again = feed(back, 2, 70, 1);
  check(again < raised && again >= raised * 0.75, "a second time at level 2 cuts less than the first", again);
  feed(back, 1, 70, 320);
  double tried = back.limit();
  check(tried > raised * 0.9, "after five minutes without level 2 it climbs past nine tenths of the rate that was too hot", tried);
  double forgot = feed(back, 1, 70, 900);
  check(forgot == 0, "and a limit back at the game's own rate is removed, at level 1 too", forgot);

  ThermalPacer cooled;
  feed(cooled, 0, 70, 60);
  feed(cooled, 2, 70, 30);
  double returning = feed(cooled, 0, 70, 25);
  check(returning > 0, "level 0 does not drop the limit at once", returning);
  double none = feed(cooled, 0, 70, 120);
  check(none == 0, "level 0 gives the limit back in steps and then removes it", none);

  ThermalPacer loading;
  feed(loading, 0, 60, 30);
  feed(loading, 0, 4, 30);
  check(feed(loading, 2, 4, 60) == 0, "a game slower than the floor at level 2 (it is loading) gets no limit", loading.limit());
  double resumed = feed(loading, 2, 90, 3);
  check(resumed > 40, "and gets one from the rate it runs at once it runs", resumed);

  ThermalPacer paused;
  feed(paused, 0, 60, 30);
  feed(paused, 2, 0, 5);
  check(paused.limit() == 0 || paused.limit() >= ThermalPacer::kFloor, "a second without frames is not a frame rate", paused.limit());
}

struct Mac {
  double temperature = 0;
  double ceiling = 70;
  double free_rate = 70;

  unsigned
  level() const {
    return temperature >= 1.0 ? 2 : temperature >= 0.79 ? 1 : 0;
  }

  double
  second(double limit) {
    double rate = std::min(free_rate, ceiling);
    if (limit > 0)
      rate = std::min(rate, limit);
    temperature += 0.012 * (rate / 40.0 - temperature);
    if (temperature > 1.0)
      ceiling = std::max(14.0, ceiling - 5.0 * 70.0 * (temperature - 1.0) * 0.07);
    else
      ceiling = std::min(free_rate, ceiling + 0.5);
    return rate;
  }
};

static void
model(bool trace) {
  Mac unheld;
  double unheld_low = 1e9;
  for (unsigned t = 0; t < 1800; t++)
    unheld_low = std::min(unheld_low, unheld.second(0));
  check(unheld_low < 30, "the model collapses without a limit, as the Mac did", unheld_low);

  Mac mac;
  ThermalPacer pacer;
  double rate = 70, low_after_settling = 1e9, sum = 0;
  unsigned heavy_seconds = 0, counted = 0;
  for (unsigned t = 0; t < 1800; t++) {
    double limit = pacer.update(mac.level(), rate);
    rate = mac.second(limit);
    if (trace && t % 15 == 0)
      printf("%5u s  level %u  temperature %.2f  limit %5.1f  rate %5.1f\n", t, mac.level(), mac.temperature, limit, rate);
    if (t < 300)
      continue;
    low_after_settling = std::min(low_after_settling, rate);
    sum += rate;
    counted++;
    heavy_seconds += mac.level() >= 2;
  }
  check(low_after_settling >= 30, "with the limit the model stays at 30 or more after five minutes", low_after_settling);
  check(sum / counted >= 34 && sum / counted <= 40, "and averages close to the 40 it can hold", sum / counted);
  check(heavy_seconds < counted / 4, "and is at level 2 less than a quarter of the time", heavy_seconds);
  if (trace)
    printf("after 300 s: lowest %.1f, mean %.1f, %u of %u s at level 2; without a limit the lowest is %.1f\n",
           low_after_settling, sum / counted, heavy_seconds, counted, unheld_low);
}

int
main(int argc, char **argv) {
  scripted();
  model(argc > 1 && !strcmp(argv[1], "trace"));
  if (failures)
    return 1;
  printf("PASS: the thermal pacer\n");
  return 0;
}
