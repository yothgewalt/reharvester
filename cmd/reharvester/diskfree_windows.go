//go:build windows

package main

// freeGB is unimplemented on Windows. Reporting false makes the Doctor skip the
// row entirely, which is honest — a fabricated number would be worse than an
// absent check.
func freeGB(string) (float64, bool) { return 0, false }
