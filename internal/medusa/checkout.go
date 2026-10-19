package medusa

import (
	"context"
	"errors"
	"fmt"
	"slices"
)

const SystemPaymentProvider = "pp_system_default"

type Line struct {
	SKU       string `json:"sku"`
	VariantID string `json:"variant_id,omitempty"`
	Quantity  int    `json:"quantity"`
}

type Target struct {
	RegionID string            `json:"region_id"`
	Variants map[string]string `json:"variants"`
}

func (c *Client) FindTarget(ctx context.Context, currency, country string, skus []string) (Target, error) {
	t := Target{Variants: map[string]string{}}

	regions, err := c.ListRegions(ctx)
	if err != nil {
		return t, err
	}
	for _, r := range regions {
		if r.CurrencyCode != currency {
			continue
		}
		for _, ct := range r.Countries {
			if ct.Iso2 == country {
				if t.RegionID != "" {
					return t, fmt.Errorf("more than one region with currency %s and country %s", currency, country)
				}
				t.RegionID = r.ID
			}
		}
	}
	if t.RegionID == "" {
		return t, fmt.Errorf("no region with currency %s and country %s", currency, country)
	}

	products, err := c.ListProducts(ctx)
	if err != nil {
		return t, err
	}
	for _, p := range products {
		for _, v := range p.Variants {
			if !slices.Contains(skus, v.SKU) {
				continue
			}
			if _, dup := t.Variants[v.SKU]; dup {
				return t, fmt.Errorf("SKU %s is on more than one variant", v.SKU)
			}
			t.Variants[v.SKU] = v.ID
		}
	}
	for _, sku := range skus {
		if t.Variants[sku] == "" {
			return t, fmt.Errorf("no variant with SKU %s in the Store API", sku)
		}
	}
	return t, nil
}

type CartInput struct {
	RegionID string
	Lines    []Line
	Email    string
	Address  Address
}

func BuyerAddress() Address {
	return Address{
		FirstName:   "Test",
		LastName:    "Buyer",
		Address1:    "1 Main St",
		City:        "Austin",
		Province:    "tx",
		PostalCode:  "78701",
		CountryCode: "us",
	}
}

func (c *Client) PrepareCart(ctx context.Context, in CartInput) (string, error) {
	cart, err := c.CreateCart(ctx, in.RegionID)
	if err != nil {
		return "", err
	}
	for _, l := range in.Lines {
		if _, err := c.AddLineItem(ctx, cart.ID, l.VariantID, l.Quantity); err != nil {
			return cart.ID, err
		}
	}
	if _, err := c.SetContact(ctx, cart.ID, in.Email, in.Address); err != nil {
		return cart.ID, err
	}
	options, err := c.ListShippingOptions(ctx, cart.ID)
	if err != nil {
		return cart.ID, err
	}
	if len(options) == 0 {
		return cart.ID, errors.New("list_shipping_options: no shipping options for cart")
	}
	if _, err := c.AddShippingMethod(ctx, cart.ID, options[0].ID); err != nil {
		return cart.ID, err
	}
	pc, err := c.CreatePaymentCollection(ctx, cart.ID)
	if err != nil {
		return cart.ID, err
	}
	if err := c.CreatePaymentSession(ctx, pc.ID, SystemPaymentProvider); err != nil {
		return cart.ID, err
	}
	return cart.ID, nil
}
