package medusa

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
)

const SystemPaymentProvider = "pp_system_default"

type Line struct {
	SKU       string `json:"sku"`
	VariantID string `json:"variant_id,omitempty"`
	Quantity  int    `json:"quantity"`
}

func ParseLines(s string) ([]Line, error) {
	if strings.TrimSpace(s) == "" {
		return nil, errors.New("no lines given")
	}
	var lines []Line
	seen := map[string]bool{}
	for _, part := range strings.Split(s, ",") {
		sku, qty, ok := strings.Cut(strings.TrimSpace(part), ":")
		if !ok || sku == "" {
			return nil, fmt.Errorf("line %q: want SKU:QUANTITY", part)
		}
		n, err := strconv.Atoi(qty)
		if err != nil || n < 1 {
			return nil, fmt.Errorf("line %q: quantity must be a positive integer", part)
		}
		if seen[sku] {
			return nil, fmt.Errorf("line %q: SKU %s given twice", part, sku)
		}
		seen[sku] = true
		lines = append(lines, Line{SKU: sku, Quantity: n})
	}
	return lines, nil
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

type CheckoutInput struct {
	RegionID string
	Lines    []Line
	Email    string
	Address  Address
}

type CheckoutResult struct {
	Completed  bool   `json:"completed"`
	CartID     string `json:"cart_id,omitempty"`
	OrderID    string `json:"order_id,omitempty"`
	FailedStep string `json:"failed_step,omitempty"`
	Error      string `json:"error,omitempty"`
}

func (c *Client) Checkout(ctx context.Context, in CheckoutInput) CheckoutResult {
	var res CheckoutResult
	fail := func(step string, err error) CheckoutResult {
		res.FailedStep = step
		res.Error = err.Error()
		return res
	}

	cart, err := c.CreateCart(ctx, in.RegionID)
	if err != nil {
		return fail("create_cart", err)
	}
	res.CartID = cart.ID

	for _, l := range in.Lines {
		if _, err := c.AddLineItem(ctx, cart.ID, l.VariantID, l.Quantity); err != nil {
			return fail("add_line_item", err)
		}
	}

	if _, err := c.SetContact(ctx, cart.ID, in.Email, in.Address); err != nil {
		return fail("set_contact", err)
	}

	options, err := c.ListShippingOptions(ctx, cart.ID)
	if err != nil {
		return fail("list_shipping_options", err)
	}
	if len(options) == 0 {
		return fail("list_shipping_options", errors.New("no shipping options for cart"))
	}
	if _, err := c.AddShippingMethod(ctx, cart.ID, options[0].ID); err != nil {
		return fail("add_shipping_method", err)
	}

	pc, err := c.CreatePaymentCollection(ctx, cart.ID)
	if err != nil {
		return fail("create_payment_collection", err)
	}
	if err := c.CreatePaymentSession(ctx, pc.ID, SystemPaymentProvider); err != nil {
		return fail("create_payment_session", err)
	}

	done, err := c.CompleteCart(ctx, cart.ID)
	if err != nil {
		return fail("complete_cart", err)
	}
	if done.Type != "order" || done.Order == nil {
		msg := "complete returned type " + strconv.Quote(done.Type)
		if done.Error != nil {
			msg += ": " + done.Error.Message
		}
		return fail("complete_cart", errors.New(msg))
	}
	res.Completed = true
	res.OrderID = done.Order.ID
	return res
}
