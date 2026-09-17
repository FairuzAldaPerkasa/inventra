package product

import "fmt"

type UpdateRequest struct {
	SKU      string `json:"sku"`
	Name     string `json:"name"`
	Category string `json:"category"`
	Brand    string `json:"brand"`
	Price    string `json:"price"`
	Version  int    `json:"version"`
}

func (req *UpdateRequest) Validate() error {
	// Gunakan aturan validasi produk yang sudah ada.
	productData := CreateRequest{
		SKU:      req.SKU,
		Name:     req.Name,
		Category: req.Category,
		Brand:    req.Brand,
		Price:    req.Price,
	}

	if err := productData.Validate(); err != nil {
		return err
	}

	// Simpan hasil normalisasi.
	req.SKU = productData.SKU
	req.Name = productData.Name
	req.Category = productData.Category
	req.Brand = productData.Brand
	req.Price = productData.Price

	if req.Version < 1 || req.Version >= 2147483647 {
		return fmt.Errorf("version harus antara 1 dan 2147483646")
	}

	return nil
}
