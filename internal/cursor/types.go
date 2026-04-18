// Package cursor provides the Cursor Agent CLI backend for story-factory.
//
// This package implements the [executor.Executor] interface by spawning
// cursor-agent as a subprocess in print mode (-p).
//
// PROVISIONAL: The cursor-agent stream-json format has not been captured yet
// (auth required). The parser currently assumes Claude-compatible JSON since
// Cursor Agent is built on similar infrastructure. This will be refined once
// real output is available.
package cursor
