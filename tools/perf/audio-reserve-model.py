"""A model of the reserve of Wine patch 0016 (runtime/wine/patches): what its
policy does against wake-up delays shaped like what Thalles's M3 showed in
Deadlock on 2026-11-19 (one 50 to 67 ms stall while the game loads, a quiet
stretch, a throttled stretch with frequent long stalls, quiet again), against
one long stall every three minutes, and against none.

    python3 -I tools/perf/audio-reserve-model.py

Prints, for the patch's first policy and the one it has now, the gaps in each
minute and the reserve at each minute's end. A model: it says how the policy
behaves, not what a Mac does. Times in ms, sizes in frames at 44100 Hz, the
client writes as SDL does.
"""
import heapq, random

RATE = 44100
PERIOD = 512

def lateness(rng, t_ms, scenario):
    base = rng.uniform(0.05, 1.5) if rng.random() < 0.97 else rng.uniform(1.5, 6.0)
    if scenario == "m3":
        s = t_ms / 1000.0
        if 4.0 <= s < 4.02:
            return 62.0                      # loading
        if 90 <= s < 300:                    # thermal level 2
            r = rng.random()
            if r < 0.004:
                return rng.uniform(90, 140)
            if r < 0.03:
                return rng.uniform(20, 60)
        return base
    if scenario == "rare":                   # one 80 ms stall every three minutes
        if int(t_ms) % 180000 < 12 and t_ms > 5000:
            return 80.0
        return base
    if scenario == "quiet":
        return base
    raise ValueError(scenario)

class First:
    """What the first test build had: starts at one period, grows by what was missing,
    gives half the unused back after five minutes without a gap."""
    def __init__(self, cap_ms=100):
        self.min = PERIOD
        self.max = cap_ms * RATE / 1000.0
        self.reserve = self.min
        self.review = 0.0
        self.low = None
    def gap(self, missing, gap_total):
        self.reserve = min(self.max, self.reserve + missing)
        self.review = 0.0
        self.low = None
    def pulled(self, frames, held):
        self.low = held if self.low is None else min(self.low, held)
        self.review += frames
        if self.review >= RATE * 300:
            if self.low is not None and self.low > self.min:
                self.reserve = max(self.min, self.reserve - (self.low - self.min) / 2)
            self.review = 0.0
            self.low = None

class Final:
    """What the patch has: five periods to start; grows by what was missing
    plus a period at a gap's start, up to 200 ms; after a minute without a
    gap (five, once there was one), each further such minute gives back half
    of what it never used; a gap after a give-back doubles that wait, up to
    40 minutes."""
    def __init__(self, cap_ms=200):
        self.min = PERIOD
        self.max = cap_ms * RATE / 1000.0
        self.reserve = min(self.max, 5 * PERIOD)
        self.review = 0.0
        self.low = None
        self.hold = 1
        self.quiet = 0
        self.gave_back = False
    def gap(self, missing, gap_total):
        if gap_total == missing:
            self.hold = min(40, max(5, self.hold) * 2) if self.gave_back else max(5, self.hold)
            self.gave_back = False
            self.quiet = 0
        want = missing + (PERIOD if gap_total == missing else 0)
        self.reserve = min(self.max, self.reserve + want)
        self.review = 0.0
        self.low = None
    def pulled(self, frames, held):
        self.low = held if self.low is None else min(self.low, held)
        self.review += frames
        if self.review >= RATE * 60:
            self.quiet += 1
            if self.quiet > self.hold and self.low > self.min and self.reserve > self.min:
                unused = (self.low - self.min) / 2
                if unused >= 1:
                    self.reserve = max(self.min, self.reserve - unused)
                    self.gave_back = True
            self.review = 0.0
            self.low = None

def run(policy, scenario, seconds=900, seed=5):
    rng = random.Random(seed)
    chunk = PERIOD
    bufsize = 3 * PERIOD
    pull_ms = PERIOD * 1000.0 / RATE
    q = []
    order = 0
    def push(t, kind):
        nonlocal order
        order += 1
        heapq.heappush(q, (t, order, kind))
    push(1.0, "pull")
    held = 0.0
    wake_pending = False
    gaps = []
    missing_by_min = {}
    gap = 0.0
    since_write = 1e12
    trace = {}
    def set_event(t, late):
        nonlocal wake_pending
        if not wake_pending:
            wake_pending = True
            push(t + late, "wake")
    while q:
        t, _, kind = heapq.heappop(q)
        if t > seconds * 1000:
            break
        minute = int(t // 60000)
        trace[minute] = policy.reserve
        if kind == "pull":
            got = min(held, PERIOD)
            held -= got
            miss = PERIOD - got
            fed = since_write < RATE / 2
            since_write += PERIOD
            if miss > 1e-6 and fed:
                if gap == 0:
                    gaps.append(minute)
                gap += miss
                missing_by_min[minute] = missing_by_min.get(minute, 0) + miss
                policy.gap(miss, gap)
            elif miss <= 1e-6:
                gap = 0.0
            policy.pulled(PERIOD, held)
            set_event(t, lateness(rng, t, scenario))
            push(t + pull_ms, "pull")
        else:
            wake_pending = False
            pad = min(max(0.0, held - policy.reserve), bufsize)
            if pad <= chunk:
                held += chunk
                since_write = 0
                if held < policy.reserve + PERIOD and max(0.0, held - policy.reserve) + PERIOD <= bufsize:
                    set_event(t, 0.05)
    minutes = seconds // 60
    return ([gaps.count(m) for m in range(minutes)],
            [round(trace.get(m, 0) * 1000 / RATE) for m in range(minutes)],
            round(sum(missing_by_min.values()) * 1000 / RATE))

if __name__ == "__main__":
    for scenario in ("m3", "rare", "quiet"):
        for name, make in (("first, cap 100 ms", First), ("now, cap 200 ms", Final)):
            gaps, reserve, missing = run(make(), scenario, seconds=1500)
            print(f"{scenario:5s} {name:17s}: {sum(gaps)} gaps {gaps}, {missing} ms missing")
            print(f"{'':24s}reserve ms {reserve}")
