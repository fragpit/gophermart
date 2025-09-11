package luhn

import (
	"strconv"
)

func ValidateNumber(num string) bool {
	sum := 0
	parity := len(num) % 2

	for i := 0; i < len(num); i++ {
		digit, err := strconv.Atoi(string(num[i]))
		if err != nil {
			return false
		}
		if i%2 == parity {
			digit *= 2
			if digit > 9 {
				digit -= 9
			}
		}
		sum += digit
	}

	return sum%10 == 0
}
