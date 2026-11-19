// Original code Copyright (c) 2026 Erik Adelbert.
// Licensed under the BSD 3-Clause license; see ../../LICENCE.
// For OpenMPT attribution and dependency licenses, see ../../pkg/mod/NOTICE.

//go:build js && wasm

// modweb plays tracker modules in a browser.
package main

import (
	"bytes"
	"errors"
	"io"
	"syscall/js"

	"github.com/erik-adelbert/duh/pkg/mod"
)

const (
	maxSourceSize         = 10 * 1024 * 1024
	durationTicksPerBatch = 64
)

func main() {
	var decoder *mod.Decoder
	var visualization *webtui
	var source io.Reader
	var duration *mod.DurationEstimator

	buf := make([]byte, 32768)

	js.Global().Set("openModule", js.FuncOf(func(_ js.Value, args []js.Value) any {
		if len(args) != 3 || !args[0].InstanceOf(js.Global().Get("Uint8Array")) ||
			args[1].Type() != js.TypeBoolean || args[2].Type() != js.TypeNumber {
			return map[string]any{"error": "Invalid module input or playback options."}
		}

		size := args[0].Get("byteLength").Int()
		if size == 0 || size > maxSourceSize {
			return map[string]any{"error": "Module files must be between 1 byte and 10 MiB."}
		}

		if decoder != nil {
			return map[string]any{"error": "A module is already open."}
		}

		data := make([]byte, size)
		js.CopyBytesToGo(data, args[0])

		var err error
		decoder, err = mod.Decode(bytes.NewReader(data), mod.DecoderOptions{
			NoLoop: args[1].Bool(), Sequence: args[2].Int(),
		})
		if err != nil {
			return map[string]any{"error": err.Error()}
		}

		source, visualization, err = newWebTUI(decoder)
		if err != nil {
			_ = decoder.Close()
			decoder = nil
			return map[string]any{"error": err.Error()}
		}
		duration, err = decoder.NewDurationEstimator()
		if err != nil {
			visualization.Stop()
			_ = decoder.Close()
			decoder, visualization, source = nil, nil, nil
			return map[string]any{"error": err.Error()}
		}
		visualization.firstPass = !args[1].Bool()

		warnings := js.Global().Get("Array").New()
		for _, warning := range decoder.Warnings() {
			warnings.Call("push", warning)
		}

		return map[string]any{"warnings": warnings}
	}))

	js.Global().Set("stepModuleDuration", js.FuncOf(func(_ js.Value, _ []js.Value) any {
		if duration == nil || visualization == nil {
			return map[string]any{"error": "No duration calculation is active."}
		}
		value, done, err := duration.Advance(durationTicksPerBatch)
		if err != nil {
			duration = nil
			return map[string]any{"error": err.Error()}
		}
		if done {
			visualization.duration = value
			visualization.hasDuration = true
			duration = nil
		}
		return map[string]any{"done": done}
	}))

	js.Global().Set("readModule", js.FuncOf(func(_ js.Value, args []js.Value) any {
		if decoder == nil {
			return map[string]any{"error": "No module is open."}
		}

		if len(args) != 1 || !args[0].InstanceOf(js.Global().Get("Uint8Array")) {
			return map[string]any{"error": "Invalid audio buffer."}
		}

		size := args[0].Get("byteLength").Int()
		if size == 0 || size > 32768 || size%8 != 0 {
			return map[string]any{"error": "Invalid audio buffer size."}
		}

		n, err := source.Read(buf[:size])
		if err != nil && !errors.Is(err, io.EOF) {
			return map[string]any{"error": err.Error()}
		}

		js.CopyBytesToJS(args[0], buf[:n])
		return map[string]any{"bytes": n, "eof": errors.Is(err, io.EOF)}
	}))

	js.Global().Set("closeModule", js.FuncOf(func(_ js.Value, _ []js.Value) any {
		if decoder == nil {
			return nil
		}

		visualization.Stop()
		err := decoder.Close()
		decoder = nil
		source, visualization, duration = nil, nil, nil

		if err != nil {
			return err.Error()
		}

		return nil
	}))

	js.Global().Set("renderModule", js.FuncOf(func(_ js.Value, args []js.Value) any {
		if visualization == nil || len(args) != 1 || args[0].Type() != js.TypeNumber {
			return ""
		}
		return visualization.Render(args[0].Float())
	}))

	js.Global().Call("moduleReady")

	select {}
}
