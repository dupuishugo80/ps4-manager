package rpi

import "testing"

func TestIsExistsResponseFound(t *testing.T) {
	tests := []struct {
		name string
		in   IsExistsResponse
		want bool
	}{
		{"exists true", IsExistsResponse{Exists: "true"}, true},
		{"exists false", IsExistsResponse{Exists: "false"}, false},
		{"unknown value", IsExistsResponse{Exists: "maybe"}, false},
		{"empty", IsExistsResponse{}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.in.Found(); got != tc.want {
				t.Fatalf("Found() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestAPIErrorMessage(t *testing.T) {
	tests := []struct {
		name string
		err  APIError
		want string
	}{
		{"message only", APIError{Message: "boom"}, "rpi: boom"},
		{"error code only", APIError{ErrorCode: 0x80990018}, "rpi: error_code=0x80990018"},
		{"message wins", APIError{Message: "boom", ErrorCode: 0x80990018}, "rpi: boom"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.err.Error(); got != tc.want {
				t.Fatalf("Error() = %q, want %q", got, tc.want)
			}
		})
	}
}
