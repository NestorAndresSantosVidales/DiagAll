package ai

import (
	"context"
	"fmt"
	"testing"
)

func TestValidation(t *testing.T) {
	m := NewLocalManager("../..") // Paths relative to internal/ai

	targets := []struct {
		name string
		data string
	}{
		{
			name: "Google (Success)",
			data: "Pinging google.com [142.251.132.14]\nRespuesta desde 142.251.132.14: tiempo=3ms TTL=119",
		},
		{
			name: "WellnestFamily (Timeout)",
			data: "Haciendo ping a wellnestfamily.com\nDNS request timed out.\nLa solicitud de ping no pudo encontrar el host.",
		},
	}

	for _, tc := range targets {
		fmt.Printf("\n--- Validating Target: %s ---\n", tc.name)
		result, err := m.Analyze(context.Background(), tc.data)
		if err != nil {
			t.Errorf("Failed to analyze %s: %v", tc.name, err)
		}
		fmt.Println(result)
	}
}
