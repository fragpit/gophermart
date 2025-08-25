package luhn

import (
	"strconv"
)

func ValidateNumber(num int) bool {
	numS := strconv.Itoa(num)
	sum := 0
	parity := len(numS) % 2

	for i := 0; i < len(numS); i++ {
		digit, _ := strconv.Atoi(string(numS[i]))
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
