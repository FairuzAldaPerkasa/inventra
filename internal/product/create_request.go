package product

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

type CreateRequest struct {
	SKU      string `json:"sku"`
	Name     string `json:"name"`
	Category string `json:"category"`
	Brand    string `json:"brand"`
	Price    string `json:"price"`
}

var (
	skuPattern   = regexp.MustCompile(`^[A-Z0-9][A-Z0-9_-]{0,49}$`)
	pricePattern = regexp.MustCompile(`^(0|[1-9][0-9]{0,12})(\.[0-9]{1,2})?$`)
)

func (req *CreateRequest) Validate() error {
	req.SKU = strings.ToUpper(strings.TrimSpace(req.SKU))
	req.Name = strings.TrimSpace(req.Name)
	req.Category = strings.TrimSpace(req.Category)
	req.Brand = strings.TrimSpace(req.Brand)
	req.Price = strings.TrimSpace(req.Price)

	if !skuPattern.MatchString(req.SKU) {
		return fmt.Errorf(
			"sku harus berisi 1–50 karakter: huruf, angka, underscore, atau tanda hubung",
		)
	}

	for _, field := range []struct {
		name  string
		value string
		max   int
	}{
		{"name", req.Name, 150},
		{"category", req.Category, 100},
		{"brand", req.Brand, 100},
	} {
		length := utf8.RuneCountInString(field.value)

		if length == 0 || length > field.max {
			return fmt.Errorf(
				"%s harus berisi 1–%d karakter",
				field.name,
				field.max,
			)
		}

		if strings.ContainsRune(field.value, '\x00') {
			return fmt.Errorf("%s mengandung karakter yang tidak valid", field.name)
		}
	}

	if !pricePattern.MatchString(req.Price) {
		return fmt.Errorf(
			"price harus berupa string angka nonnegatif, maksimal 13 digit sebelum koma dan 2 digit desimal; gunakan titik sebagai pemisah",
		)
	}

	return nil
}
