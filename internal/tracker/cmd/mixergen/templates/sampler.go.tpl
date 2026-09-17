{{- /* Sampler function template */ -}}

{{/* --------------------------- */}}
{{/* Comment template */}}

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
// {{ .Sampler }} yields {{ $channels }} {{ $bitDepth }}-bit samples using {{ $interpolation }} interpolation.
// This function reads from buf, applies {{ $interpolation }} interpolation (and filtering if applicable),
// and yields the resulting sample(s) as Fp284 values. The cue is updated with the new playback position.
// The function assumes buf contains valid {{ $bitDepth }}-bit {{ $channels }} PCM data and is large enough
// for the requested operation.
// It returns an iterator (Seq or Seq2) yielding one (mono) or two (stereo) Fp284 samples per call.
{{ end }}

{{/* --------------------------- */}}
{{/* Emit yield and filter code snippets */}}

{{ define "emit_yield" }}
{{- if .Stereo -}}
    if !yield(l, r) { break }
{{- else -}}
    if !yield(l) { break }
{{- end -}}
{{ end }}

{{ define "emit_filter" }}
{{- if .Filter -}}
    l = (l*A0 + y1*B0 + y2*B1 + 4096) >> 13
    r = (r*A0 + y3*B0 + y4*B1 + 4096) >> 13

    y2, y1 = y1, l
    y4, y3 = y3, r
{{ end -}}
{{ end }}

{{/* --------------------------- */}}
{{/* 8-bit (mono/stereo) */}}

{{ define "basic8" }}
{{ if .Stereo }}
    step := 2 * cue.Step
    bufLen := len(buf) / 2
{{ else }}
    step := cue.Step
    bufLen := len(buf)
{{ end }}

for i = cue.FPart; hi(i) < bufLen; i += step {
    idx := hi(i)

    {{ if .Stereo }}
        l := Fp284(buf[idx*2]) << 8
        r := Fp284(buf[idx*2+1]) << 8
    {{ else }}
        l := Fp284(buf[idx]) << 8
        r := Fp284(0); _ = r // generation sugar for mono
    {{ end }}

    {{ template "emit_filter" . }}
    {{ template "emit_yield" . }}
}
{{ end }}

{{/* --------------------------- */}}
{{/* 16-bit (mono/stereo) */}}

{{ define "basic16" }}
{{ if .Stereo }}
    step := 4 * cue.Step
    bufLen := len(buf) / 4
{{ else }}
    step := 2 * cue.Step
    bufLen := len(buf) / 2
{{ end }}

for i = cue.FPart; hi(i) < bufLen; i += step {
    idx := hi(i)

    {{ if .Stereo }}
        l := Fp284(buf[idx*4]) | Fp284(buf[idx*4+1])<<8
        r := Fp284(buf[idx*4+2]) | Fp284(buf[idx*4+3])<<8
    {{ else }}
        l := Fp284(buf[idx*2]) | Fp284(buf[idx*2+1])<<8
        r := Fp284(0); _ = r // generation sugar for mono
    {{ end }}

    {{ template "emit_filter" . }}
    {{ template "emit_yield" . }}
}
{{ end }}

{{/* --------------------------- */}}
{{/* Linear interpolation 8-bit (mono/stereo) */}}

{{ define "linear8" }}
{{ if .Stereo }}
    step := 2 * cue.Step
    bufLen := len(buf)/2 - 1
{{ else }}
    step := cue.Step
    bufLen := len(buf) - 1
{{ end }}

for i = cue.FPart; hi(i) < bufLen; i += step {
    idx := hi(i)
    frac := lo(i)

    {{ if .Stereo }}
        l0 := Fp284(buf[idx*2]) << 8
        l1 := Fp284(buf[idx*2+2]) << 8
        r0 := Fp284(buf[idx*2+1]) << 8
        r1 := Fp284(buf[idx*2+3]) << 8
    {{ else }}
        l0 := Fp284(buf[idx]) << 8
        l1 := Fp284(buf[idx+1]) << 8
        r0, r1 := Fp284(0), Fp284(0)
    {{ end }}

    l := l0 + ((l1-l0)*frac)>>16
    r := r0 + ((r1-r0)*frac)>>16; _ = r // generation sugar for mono

    {{ template "emit_filter" . }}
    {{ template "emit_yield" . }}
}
{{ end }}

{{/* --------------------------- */}}
{{/* Linear interpolation 16-bit (mono/stereo) */}}

{{ define "linear16" }}
{{ if .Stereo }}
    step := 4 * cue.Step
    bufLen := len(buf)/4 - 1
{{ else }}
    step := 2 * cue.Step
    bufLen := len(buf)/2 - 1
{{ end }}

for i = cue.FPart; hi(i) < bufLen; i += step {
    idx := hi(i)
    frac := lo(i)

    {{ if .Stereo }}
        l0 := Fp284(buf[idx*4]) | Fp284(buf[idx*4+1])<<8
        l1 := Fp284(buf[idx*4+4]) | Fp284(buf[idx*4+5])<<8
        r0 := Fp284(buf[idx*4+2]) | Fp284(buf[idx*4+3])<<8
        r1 := Fp284(buf[idx*4+6]) | Fp284(buf[idx*4+7])<<8
    {{ else }}
        l0 := Fp284(buf[idx*2]) | Fp284(buf[idx*2+1])<<8
        l1 := Fp284(buf[idx*2+2]) | Fp284(buf[idx*2+3])<<8
        r0, r1 := Fp284(0), Fp284(0)
    {{ end }}

    l := l0 + ((l1-l0)*frac)>>16
    r := r0 + ((r1-r0)*frac)>>16; _ = r // generation sugar for mono

    {{ template "emit_filter" . }}
    {{ template "emit_yield" . }}
}
{{ end }}

{{/* --------------------------- */}}
{{/* Spline interpolation 8-bit (mono/stereo) */}}

{{ define "spline8" }}
{{ if .Stereo }}
    step := 2 * cue.Step
    bufLen := len(buf)/2 - 3
{{ else }}
    step := cue.Step
    bufLen := len(buf) - 3
{{ end }}

for i = cue.FPart; hi(i) < bufLen; i += step {
    idx := hi(i)
    frac := lo(i) >> (16 - csFracBits)

    var l, r Fp284

    for j := range 4 {
        j0 := int(frac)*4 + j
        cs := Fp284(csLUT[j0])

        {{ if .Stereo }}
            l += cs * (Fp284(buf[(idx+j)*2]) << 8)
            r += cs * (Fp284(buf[(idx+j)*2+1]) << 8)
        {{ else }}
            l += cs * (Fp284(buf[idx+j]) << 8)
            r = 0
        {{ end -}}
    }

    l >>= csShift16
    {{ if not .Stereo }} _ = r // generation sugar for mono {{ end }}

    {{ template "emit_filter" . }}
    {{ template "emit_yield" . }}
}
{{ end }}

{{/* --------------------------- */}}
{{/* Spline interpolation 16-bit (mono/stereo) */}}

{{ define "spline16" }}
{{ if .Stereo }}
    step := 4 * cue.Step
    bufLen := len(buf)/4 - 3
{{ else }}
    step := 2 * cue.Step
    bufLen := len(buf)/2 - 3
{{ end }}

for i = cue.FPart; hi(i) < bufLen; i += step {
    idx := hi(i)
    frac := lo(i) >> (16 - csFracBits)

    var l, r Fp284

    for j := range 4 {
        j0 := int(frac)*4 + j
        cs := Fp284(csLUT[j0])

        {{ if .Stereo }}
            l += cs * (Fp284(buf[(idx+j)*4]) | Fp284(buf[(idx+j)*4+1])<<8)
            r += cs * (Fp284(buf[(idx+j)*4+2]) | Fp284(buf[(idx+j)*4+3])<<8)
        {{ else }}
            l += cs * (Fp284(buf[(idx+j)*2]) | Fp284(buf[(idx+j)*2+1])<<8)
            r = 0
        {{ end -}}
    }

    l >>= csShift16
    {{ if not .Stereo }} _ = r // generation sugar for mono {{ end }}

    {{ template "emit_filter" . }}
    {{ template "emit_yield" . }}
}
{{ end }}

{{/* --------------------------- */}}
{{/* FIR interpolation 8-bit (mono/stereo) */}}

{{ define "fir8" }}
{{ if .Stereo }}
    step := 2 * cue.Step
    bufLen := len(buf)/2 - wfWidth + 1
{{ else }}
    step := cue.Step
    bufLen := len(buf) - wfWidth + 1
{{ end }}

for i = cue.FPart; hi(i) < bufLen; i += step {
    idx := hi(i)
    frac := lo(i) >> (16 - wfFracBits)

    var l, r Fp284
    j0 := int(frac * wfWidth)

    for j := range wfWidth {
        k := Fp284(wfLUT[j0+j])

        {{ if .Stereo }}
            l += k * Fp284(buf[(idx+j)*2])
            r += k * Fp284(buf[(idx+j)*2+1])
        {{ else }}
            l += k * Fp284(buf[idx+j])
        {{ end -}}
    }

    l >>= wfShift16
    {{ if not .Stereo }} _ = r // generation sugar for mono {{ end }}

    {{ template "emit_filter" . }}
    {{ template "emit_yield" . }}
}
{{ end }}

{{/* --------------------------- */}}
{{/* FIR interpolation 16-bit (mono/stereo) */}}

{{ define "fir16" }}
{{ if .Stereo }}
    step := 4 * cue.Step
    bufLen := len(buf)/4 - wfWidth + 1
{{ else }}
    step := 2 * cue.Step
    bufLen := len(buf)/2 - wfWidth + 1
{{ end }}

for i = cue.FPart; hi(i) < bufLen; i += step {
    idx := hi(i)
    frac := lo(i) >> (16 - wfFracBits)

    var l, r Fp284
    j0 := int(frac * wfWidth)

    for j := range wfWidth {
        k := Fp284(wfLUT[j0+j])
        {{ if .Stereo }}
            l += k * (Fp284(buf[(idx+j)*4]) | Fp284(buf[(idx+j)*4+1])<<8)
            r += k * (Fp284(buf[(idx+j)*4+2]) | Fp284(buf[(idx+j)*4+3])<<8)
        {{ else }}
            l += k * (Fp284(buf[(idx+j)*2]) | Fp284(buf[(idx+j)*2+1])<<8)
        {{ end -}}
    }

    l >>= wfShift16
    {{ if not .Stereo }} _ = r // generation sugar for mono {{ end }}

    {{ template "emit_filter" . }}
    {{ template "emit_yield" . }}
}
{{ end }}

{{/* --------------------------- */}}
{{/* Sampler function template */}}

{{ range . }}

{{- $suf := "8" -}}
{{- if .Bit16 -}}
    {{- $suf = "16" -}}
{{- end -}}

{{- $seq := "iter.Seq" -}}
{{- $typ := "Fp284" -}}
{{- if .Stereo -}}
    {{- $seq = "iter.Seq2" -}}
    {{- $typ = "Fp284, Fp284" -}}
{{- end -}}

{{- $sampler := printf "basic%s" $suf -}}
{{- if .Linear -}}
    {{- $sampler = printf "linear%s" $suf -}}
{{- else if .Spline -}}
    {{- $sampler = printf "spline%s" $suf -}}
{{- else if and .FIR -}}
    {{- $sampler = printf "fir%s" $suf -}}
{{- end -}}

{{ template "comment" . -}}
func {{ .Sampler }}(buf []int8, cue *ac.Cue, filter *ac.Filter) {{ $seq }}[{{ $typ }}] {
    return func(yield func({{ $typ }}) bool) {
        var i int32

        {{ if .Filter }}
        A0, B0, B1 := filter.A0, filter.B0, filter.B1
        y1, y2 := filter.Y[0], filter.Y[1]
        y3, y4 := filter.Y[2], filter.Y[3]
        {{ end }}

        {{if .Linear -}}
            {{- if .Bit16 -}}
                {{ template "linear16" . }}
            {{- else -}}
                {{ template "linear8" . }}
            {{- end -}}
        {{- else if .Spline -}}
            {{- if .Bit16 -}}
                {{ template "spline16" . }}
            {{- else -}}
                {{ template "spline8" . }}
            {{- end -}}
        {{- else if .FIR -}}
            {{- if .Bit16 -}}
                {{ template "fir16" . }}
            {{- else -}}
                {{ template "fir8" . }}
            {{- end -}}
        {{- else -}}
            {{- if .Bit16 -}}
                {{ template "basic16" . }}
            {{- else -}}
                {{ template "basic8" . }}
            {{- end -}}
        {{- end}}

        {{ if .Filter }}
        filter.Y = [4]Fp284{y1, y2, y3, y4}
        {{ end }}

        cue.IPart = int32(hi(i))
        cue.FPart = int32(lo(i))
    }
}

{{ end }}

func hi(i int32) int { return int(i >> 16) }

func lo(i int32) Fp284 { return Fp284(i & 0xFFFF) }