/* What a Windows program sees of AVX under Wine and Rosetta, and what AVX
 * code costs there against SSE code doing the same work.
 *
 * Prints RESULT lines: the CPUID bits a game reads (AVX, AVX2, FMA, F16C,
 * BMI1/2, OSXSAVE and the XCR0 YMM bit), Windows' own answer
 * (IsProcessorFeaturePresent, GetEnabledXStateFeatures), then times one
 * float kernel (y = a*x + y over 4096 floats, kept in L1, repeated) built
 * as SSE2 and as AVX2+FMA. The AVX2 kernel runs only when CPUID says AVX2
 * and the OS saves YMM, as a game would decide.
 *
 * The lines also go to the file named by the first argument (default
 * avxprobe.txt in the current folder): a program started through Macron's
 * steam.exe has no stdout that reaches macron's log.
 *
 * Build: x86_64-w64-mingw32-gcc -O2 -o avxprobe.exe avxprobe.c */
#include <windows.h>
#include <cpuid.h>
#include <immintrin.h>
#include <stdarg.h>
#include <stdio.h>

#define N 4096
#define REPEATS 200000

static float xs[N] __attribute__((aligned(32)));
static float ys[N] __attribute__((aligned(32)));

__attribute__((target("sse2"), noinline)) static void saxpy_sse2(float a, int repeats)
{
    __m128 va = _mm_set1_ps(a);
    for (int r = 0; r < repeats; r++)
        for (int i = 0; i < N; i += 4) {
            __m128 y = _mm_load_ps(ys + i);
            y = _mm_add_ps(y, _mm_mul_ps(va, _mm_load_ps(xs + i)));
            _mm_store_ps(ys + i, y);
        }
}

__attribute__((target("avx2,fma"), noinline)) static void saxpy_avx2(float a, int repeats)
{
    __m256 va = _mm256_set1_ps(a);
    for (int r = 0; r < repeats; r++)
        for (int i = 0; i < N; i += 8) {
            __m256 y = _mm256_load_ps(ys + i);
            y = _mm256_fmadd_ps(va, _mm256_load_ps(xs + i), y);
            _mm256_store_ps(ys + i, y);
        }
    _mm256_zeroupper();
}

__attribute__((target("xsave"))) static unsigned long long xcr0(void)
{
    return _xgetbv(0);
}

static double seconds(LARGE_INTEGER from, LARGE_INTEGER to, LARGE_INTEGER freq)
{
    return (double)(to.QuadPart - from.QuadPart) / (double)freq.QuadPart;
}

static void reset(void)
{
    for (int i = 0; i < N; i++) {
        xs[i] = (float)(i % 17) * 0.25f;
        ys[i] = 0.0f;
    }
}

static FILE *result_file;

static void result(const char *format, ...)
{
    va_list args;
    va_start(args, format);
    vprintf(format, args);
    va_end(args);
    if (result_file) {
        va_start(args, format);
        vfprintf(result_file, format, args);
        va_end(args);
        fflush(result_file);
    }
}

typedef DWORD64 (WINAPI *get_xstate_fn)(void);

int main(int argc, char **argv)
{
    result_file = fopen(argc > 1 ? argv[1] : "avxprobe.txt", "w");
    unsigned a, b, c, d;
    __cpuid(0, a, b, c, d);
    unsigned max_leaf = a;
    __cpuid(1, a, b, c, d);
    int sse42 = (c >> 20) & 1, avx = (c >> 28) & 1, fma = (c >> 12) & 1, f16c = (c >> 29) & 1, osxsave = (c >> 27) & 1;
    int avx2 = 0, bmi1 = 0, bmi2 = 0, avx512f = 0;
    if (max_leaf >= 7) {
        __cpuid_count(7, 0, a, b, c, d);
        avx2 = (b >> 5) & 1;
        bmi1 = (b >> 3) & 1;
        bmi2 = (b >> 8) & 1;
        avx512f = (b >> 16) & 1;
    }
    unsigned long long x = osxsave ? xcr0() : 0;
    int ymm_saved = (x & 6) == 6;
    result("RESULT cpuid sse4.2=%d avx=%d avx2=%d fma=%d f16c=%d bmi1=%d bmi2=%d avx512f=%d osxsave=%d xcr0=0x%llx\n",
           sse42, avx, avx2, fma, f16c, bmi1, bmi2, avx512f, osxsave, x);

    get_xstate_fn get_xstate = (get_xstate_fn)GetProcAddress(GetModuleHandleA("kernel32.dll"), "GetEnabledXStateFeatures");
    result("RESULT windows PF_AVX=%d PF_AVX2=%d PF_AVX512F=%d xstate=0x%llx\n",
           IsProcessorFeaturePresent(39), IsProcessorFeaturePresent(40), IsProcessorFeaturePresent(41),
           get_xstate ? (unsigned long long)get_xstate() : 0ULL);

    LARGE_INTEGER freq, t0, t1;
    QueryPerformanceFrequency(&freq);
    double flops = 2.0 * N * (double)REPEATS;

    reset();
    saxpy_sse2(1.0001f, 1000);
    QueryPerformanceCounter(&t0);
    saxpy_sse2(1.0001f, REPEATS);
    QueryPerformanceCounter(&t1);
    double sse = seconds(t0, t1, freq);
    float check_sse = ys[N - 1];
    result("RESULT sse2 %.3f s %.2f GFLOP/s\n", sse, flops / sse / 1e9);

    if (avx2 && fma && ymm_saved) {
        reset();
        saxpy_avx2(1.0001f, 1000);
        QueryPerformanceCounter(&t0);
        saxpy_avx2(1.0001f, REPEATS);
        QueryPerformanceCounter(&t1);
        double v = seconds(t0, t1, freq);
        result("RESULT avx2 %.3f s %.2f GFLOP/s, %.2fx the SSE2 time (results %s)\n", v, flops / v / 1e9, v / sse,
               ys[N - 1] == check_sse ? "equal" : "differ (FMA rounds once)");
    } else {
        result("RESULT avx2 skipped: not advertised\n");
    }
    fflush(stdout);
    return 0;
}
