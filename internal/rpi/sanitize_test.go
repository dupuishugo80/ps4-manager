package rpi

import "testing"

func TestSanitizeHexNumbers(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"no hex", `{"status":"success"}`, `{"status":"success"}`},
		{"single hex", `{"size": 0xFF}`, `{"size": 255}`},
		{
			"multiple hex",
			`{"length": 0xFD4C65000, "transferred": 0x100}`,
			`{"length": 67994275840, "transferred": 256}`,
		},
		{"uppercase prefix", `{"a": 0X10}`, `{"a": 16}`},
		{"lowercase digits", `{"a": 0xabcd}`, `{"a": 43981}`},
		{"zero value", `{"a": 0x0}`, `{"a": 0}`},
		{"hex in string untouched", `{"msg": "0xFF is hex"}`, `{"msg": "0xFF is hex"}`},
		{
			"full progress payload",
			`{"status":"success","bits":0xAB,"error":0,"length":0xFD4C65000,"num_index":1}`,
			`{"status":"success","bits":171,"error":0,"length":67994275840,"num_index":1}`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := string(sanitizeHexNumbers([]byte(tc.in)))
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}
