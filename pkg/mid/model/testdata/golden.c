/*
 * Golden-value generator for the Go volume/frequency models.
 *
 * Build and run against a libADLMIDI checkout:
 *
 *   cc -I$ADLMIDI/src/models -o golden golden.c $ADLMIDI/src/models/model_*.c -lm
 *   ./golden > golden.txt
 *
 * Output lines:
 *   F <model> <tone*4> <offset> <value>
 *   V <model> vel chVol chExpr master mode fbConn tlMod tlCar doMod doCar drum tlMod' tlCar'
 *   B <index> <value>
 */
#include <stdio.h>
#include <stdint.h>
#include "opl_models.h"

typedef uint16_t (*freqfn)(double, uint32_t *);
typedef void (*volfn)(struct OPLVolume_t *);

static const struct { const char *name; freqfn fn; } freqs[] = {
    {"generic", oplModel_genericFreq}, {"dmx", oplModel_dmxFreq},
    {"apogee", oplModel_apogeeFreq},   {"win9x", oplModel_9xFreq},
    {"hmi", oplModel_hmiFreq},         {"ail", oplModel_ailFreq},
    {"adlib", oplModel_msAdLibFreq},   {"oconnell", oplModel_OConnellFreq},
};

static const struct { const char *name; volfn fn; } vols[] = {
    {"generic", oplModel_genericVolume},       {"native", oplModel_nativeVolume},
    {"rsxx", oplModel_rsxxVolume},             {"dmx", oplModel_dmxOrigVolume},
    {"dmxfixed", oplModel_dmxFixedVolume},     {"apogee", oplModel_apogeeOrigVolume},
    {"apogeefixed", oplModel_apogeeFixedVolume}, {"win9x", oplModel_9xGenericVolume},
    {"sb16", oplModel_9xSB16Volume},           {"hmiold", oplModel_sosOldVolume},
    {"hmi", oplModel_sosNewVolume},            {"ail", oplModel_ailVolume},
    {"adlib", oplModel_msAdLibVolume},         {"oconnell", oplModel_OConnellVolume},
};

static uint32_t rng = 0x12345678u;

static uint32_t next(void)
{
    rng ^= rng << 13;
    rng ^= rng >> 17;
    rng ^= rng << 5;
    return rng;
}

int main(void)
{
    unsigned i, m, n;

    for(m = 0; m < sizeof(freqs) / sizeof(freqs[0]); m++)
    {
        for(i = 0; i <= 600; i++)
        {
            uint32_t off = 0;
            uint16_t v = freqs[m].fn(i / 4.0, &off);
            printf("F %s %u %u %u\n", freqs[m].name, i, off, v);
        }
    }

    for(m = 0; m < sizeof(vols) / sizeof(vols[0]); m++)
    {
        rng = 0x12345678u;

        for(n = 0; n < 400; n++)
        {
            struct OPLVolume_t v;
            uint32_t r = next();

            v.vel = (n % 8 == 0) ? 127 : next() % 128;
            v.chVol = (n % 7 == 0) ? 127 : next() % 128;
            v.chExpr = (n % 5 == 0) ? 127 : next() % 128;
            v.masterVolume = (n % 3 == 0) ? 127 : next() % 128;
            v.voiceMode = r % 10;
            v.fbConn = (r >> 4) % 3 == 0 ? 0 : next() % 256;
            v.tlMod = next() % 64;
            v.tlCar = next() % 64;
            v.doMod = (r >> 8) & 1;
            v.doCar = (r >> 9) & 1;
            v.isDrum = (r >> 10) & 1;

            printf("V %s %u %u %u %u %u %u %u %u %u %u %u", vols[m].name,
                   v.vel, v.chVol, v.chExpr, v.masterVolume, v.voiceMode, v.fbConn,
                   v.tlMod, v.tlCar, v.doMod, v.doCar, v.isDrum);

            vols[m].fn(&v);
            printf(" %u %u\n", v.tlMod & 63, v.tlCar & 63);
        }
    }

    for(i = 0; i < 128; i++)
        printf("B %u %u\n", i, (unsigned)oplModels_xgBrightnessToOPL(i));

    return 0;
}
