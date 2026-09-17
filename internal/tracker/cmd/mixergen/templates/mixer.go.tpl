{{- /* Mixer function template */ -}}

{{ define "comment" }}
{{- $channels := "mono" -}}
{{- if .Stereo -}}
    {{- $channels = "stereo" -}}
{{- end -}}
{{- $bitDepth := "8" -}}
{{- if .Bit16 -}}
    {{- $bitDepth = "16" -}}
{{- end -}}
{{- $interpolation := "nearest" -}}
{{- if .Linear -}}
    {{- $interpolation = "linear" -}}
{{- else if .Spline -}}
    {{- $interpolation = "spline" -}}
{{- else if .FIR -}}
    {{- $interpolation = "FIR" -}}
{{- end -}}
// {{ .Mixer }} mixes {{ $channels }} {{ $bitDepth }}-bit PCM samples using {{ $interpolation }} interpolation.
// This function applies {{ $interpolation }} interpolation to the input samples in src,
// mixes the result into the output buffer buf, and applies per-channel gain.
// It assumes buf is large enough to hold the output and that data contains
// valid {{ $bitDepth }}-bit {{ $channels }} PCM data.
{{ end }}

{{/* --------------------------- */}}
{{/* Mixer function template */}}

{{ range . }}

{{- $suf := "8" -}}
{{- if .Bit16 -}}
    {{- $suf = "16" -}}
{{- end -}}

{{ template "comment" . -}}
func {{ .Mixer }}(mixbuf []Fp284, cue *ac.Cue, gain *ac.Gain, filter *ac.Filter, data sample.Data, flags ac.Flags) {
    i := cue.FPart

    if flags&ac.VoiceStereo != 0 {
        i *= 2
    }

{{ if and .FastMono .Ramp}}
    rL := gain.Ramp.Right
    rR := gain.Ramp.Right
{{ else if .Ramp }}
    rL := gain.Ramp.Left
    rR := gain.Ramp.Right
{{ end }}

{{ if .Stereo }}
    next, stop := iter.Pull2({{ .Sampler }}(data[i:], cue, filter))
{{ else }}
    next, stop := iter.Pull({{ .Sampler }}(data[i:], cue, filter))
{{ end }}
    defer stop()

    for i := 0; i < len(mixbuf); i += 2 {
    {{- if .Stereo }}
        l, r, ok := next()
    {{- else }}
        c, ok := next()
    {{ end }}

        if !ok {
            break
        }

    {{ if and .Stereo .Ramp }}
        rL += gain.Volume.Left
        rR += gain.Volume.Right

        mixbuf[i] += l * rL
        mixbuf[i+1] += r * rR
    {{ else if and .FastMono .Ramp }}
        rL += gain.Volume.Right
        rR += gain.Volume.Right

        mixbuf[i] += c * rL
        mixbuf[i+1] += c * rR
    {{ else if .Stereo }}
        mixbuf[i] += l * gain.Volume.Left
        mixbuf[i+1] += r * gain.Volume.Right
    {{ else if .FastMono }}
        mixbuf[i] += c * gain.Volume.Right
        mixbuf[i+1] += c * gain.Volume.Right
    {{ else if .Ramp }}
        rL += gain.Volume.Left
        rR += gain.Volume.Right

        mixbuf[i] += c * rL
        mixbuf[i+1] += c * rR
    {{ else }}
        mixbuf[i] += c * gain.Volume.Left
        mixbuf[i+1] += c * gain.Volume.Right
    {{ end -}}
	}
    {{ if .Ramp }}

    gain.Ramp.Left = rL
    gain.Left = rL >> 12
    gain.Ramp.Right = rR
    gain.Right = rR >> 12
    {{ end -}}
}

{{ end }}