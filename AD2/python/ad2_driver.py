#!/usr/bin/env python3
# =============================================================================
# waves_worx/AD2 — Digilent Analog Discovery 2 hardware driver (the thin adapter)
# =============================================================================
# This is the ONLY hardware-specific file. It does no DSP and no encoding: it
# plays a list of tones out of the AWG (W1) and captures the resulting samples
# on the scope (channel 1, 1+ / 1-), then prints them as JSON. The JS side owns
# all modulation/recovery (modem.encode/decode + dsp.samplesToFreqs); this file
# is the physical hop only — exactly the "thin adapter on a pure core" pattern.
#
# PER-TARGET BEHAVIOR : real hardware only — needs an AD2 + the WaveForms runtime.
# HARDWARE            : Digilent Analog Discovery 2. Bench wiring for a loopback:
#                         W1  →  1+   (AWG output into scope channel 1 positive)
#                         ⏚   →  1-   (common ground into scope channel 1 neg)
# DEPENDENCY          : the WaveForms SDK (libdwf), installed by the WaveForms
#                       RPM on Fedora; load it via ctypes (no pip package needed).
# PERFORMANCE         : per-tone play+capture loop — NOT real-time; fine for a
#                       teaching/bench run. (A streaming AnalogOut "play" + AnalogIn
#                       "record" version is the upgrade path for live transport.)
# FALLBACK            : none here. With no device, use the JS mock instead:
#                         node bench.mjs --mock "your message"
#
# Usage:
#   echo '{"tones":[{"freqHz":1200,"durationMs":60}],"sampleRate":48000}' \
#     | python3 ad2_driver.py            # → {"sampleRate":48000,"samples":[...]}
#   python3 ad2_driver.py --probe        # → open the device, print its name, exit
# =============================================================================
import ctypes
import json
import platform
import sys
from ctypes import byref, c_byte, c_double, c_int

# ---- load libdwf across platforms -------------------------------------------
def load_dwf():
    name = platform.system()
    if name == "Windows":
        return ctypes.cdll.dwf
    if name == "Darwin":
        return ctypes.cdll.LoadLibrary("/Library/Frameworks/dwf.framework/dwf")
    return ctypes.cdll.LoadLibrary("libdwf.so")  # Linux (Fedora WaveForms RPM)

# ---- minimal dwf constants (subset of dwfconstants.py) ----------------------
FUNC_SINE = c_byte(1)
ACQMODE_SINGLE = c_int(0)
STATE_DONE = 2
NODE_CARRIER = c_int(0)
AWG_BUFFER_MAX = 16384  # AD2 per-channel buffer ceiling


def _open(dwf):
    hdwf = c_int()
    dwf.FDwfDeviceOpen(c_int(-1), byref(hdwf))
    if hdwf.value == 0:
        err = ctypes.create_string_buffer(512)
        dwf.FDwfGetLastErrorMsg(err)
        raise RuntimeError(
            "could not open the AD2 — is it plugged in and the WaveForms GUI "
            "closed? (" + err.value.decode(errors="replace").strip() + ")"
        )
    return hdwf


def probe():
    dwf = load_dwf()
    version = ctypes.create_string_buffer(32)
    dwf.FDwfGetVersion(version)
    hdwf = _open(dwf)
    try:
        return {"ok": True, "dwfVersion": version.value.decode(), "opened": True}
    finally:
        dwf.FDwfDeviceClose(hdwf)


def play_and_capture(tones, sample_rate, amplitude=1.0, ch_out=0, ch_in=0):
    """Play each tone on the AWG and capture its samples on the scope, in order.

    Returns a flat list of voltage samples (one contiguous buffer), aligned to
    the JS side's per-symbol windowing (samplesToFreqs uses the same symbolMs).
    """
    dwf = load_dwf()
    hdwf = _open(dwf)
    try:
        # Scope: single acquisition, ±5 V range, requested sample rate.
        dwf.FDwfAnalogInChannelEnableSet(hdwf, c_int(ch_in), c_int(1))
        dwf.FDwfAnalogInChannelRangeSet(hdwf, c_int(ch_in), c_double(5.0))
        dwf.FDwfAnalogInAcquisitionModeSet(hdwf, ACQMODE_SINGLE)
        dwf.FDwfAnalogInFrequencySet(hdwf, c_double(sample_rate))

        # AWG: a sine carrier whose frequency we retune per tone.
        dwf.FDwfAnalogOutNodeEnableSet(hdwf, c_int(ch_out), NODE_CARRIER, c_int(1))
        dwf.FDwfAnalogOutNodeFunctionSet(hdwf, c_int(ch_out), NODE_CARRIER, FUNC_SINE)
        dwf.FDwfAnalogOutNodeAmplitudeSet(hdwf, c_int(ch_out), NODE_CARRIER, c_double(amplitude))

        samples = []
        for tone in tones:
            n = max(16, int(round(sample_rate * float(tone["durationMs"]) / 1000.0)))
            if n > AWG_BUFFER_MAX:
                raise ValueError(
                    f"symbol window {n} samples exceeds the AD2 buffer "
                    f"({AWG_BUFFER_MAX}); lower sampleRate or symbolMs"
                )
            dwf.FDwfAnalogInBufferSizeSet(hdwf, c_int(n))
            dwf.FDwfAnalogOutNodeFrequencySet(
                hdwf, c_int(ch_out), NODE_CARRIER, c_double(float(tone["freqHz"]))
            )
            dwf.FDwfAnalogOutConfigure(hdwf, c_int(ch_out), c_int(1))  # start AWG
            dwf.FDwfAnalogInConfigure(hdwf, c_int(0), c_int(1))        # arm + start scope

            sts = c_byte()
            while True:
                dwf.FDwfAnalogInStatus(hdwf, c_int(1), byref(sts))
                if sts.value == STATE_DONE:
                    break

            buf = (c_double * n)()
            dwf.FDwfAnalogInStatusData(hdwf, c_int(ch_in), buf, c_int(n))
            samples.extend(buf[:n])

        dwf.FDwfAnalogOutReset(hdwf, c_int(ch_out))
        return samples
    finally:
        dwf.FDwfDeviceClose(hdwf)


def main(argv):
    if "--probe" in argv:
        print(json.dumps(probe()))
        return 0
    req = json.loads(sys.stdin.read() or "{}")
    samples = play_and_capture(
        req["tones"],
        float(req.get("sampleRate", 48000)),
        amplitude=float(req.get("amplitude", 1.0)),
    )
    print(json.dumps({"sampleRate": req.get("sampleRate", 48000), "samples": samples}))
    return 0


if __name__ == "__main__":
    try:
        sys.exit(main(sys.argv[1:]))
    except Exception as exc:  # noqa: BLE001 — surface a clean JSON error to the orchestrator
        print(json.dumps({"ok": False, "error": str(exc)}), file=sys.stderr)
        sys.exit(1)
