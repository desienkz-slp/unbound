package model

import "testing"

func TestValidateDesiredState_RedirectIP(t *testing.T) {
	cases := []struct {
		name    string
		adultIP string
		gambIP  string
		wantErr bool
	}{
		{"empty means NXDOMAIN", "", "", false},
		{"valid IPv4", "172.18.20.145", "172.18.20.145", false},
		{"invalid text", "not-an-ip", "", true},
		{"IPv6 rejected for A record", "", "2001:db8::1", true},
		{"URL rejected", "https://google.com", "", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			state := NewDefaultDesiredState("test-node", 4, 4096)
			state.ParentalControl.RedirectIP = c.adultIP
			state.GamblingBlock.RedirectIP = c.gambIP
			found := false
			for _, e := range ValidateDesiredState(state) {
				if e.Field == "parental_control.redirect_ip" || e.Field == "gambling_block.redirect_ip" {
					found = true
				}
			}
			if found != c.wantErr {
				t.Fatalf("redirect IP error = %v, want %v", found, c.wantErr)
			}
		})
	}
}
