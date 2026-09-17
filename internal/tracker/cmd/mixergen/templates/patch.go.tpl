{{- /* Patch routine table template */ -}}

// PatchID matches uniquely an audiochannel with a mixing routine.
// It is hashed from a combination of audio properties that are shared
// between the channel and the routine.
type PatchID = ac.PatchID

// Patch is a table of mixing routines indexed by PatchIDs.
var Patch = [...]Routine{
{{ range . }}
{{- .Index }}: {{ .Mixer }},
{{ end }}
}