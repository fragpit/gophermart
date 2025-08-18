package luhn

import "testing"

func TestValidateNumber(t *testing.T) {
	type args struct {
		num int
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "valid number",
			args: args{
				num: 79927398713,
			},
			want: true,
		},
		{
			name: "invalid number",
			args: args{
				num: 799277398713,
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ValidateNumber(tt.args.num); got != tt.want {
				t.Errorf("ValidateNumber() = %v, want %v", got, tt.want)
			}
		})
	}
}
