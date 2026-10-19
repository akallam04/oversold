package medusa

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/akallam04/oversold/internal/history"
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

type CheckoutResult struct {
	Completed  bool            `json:"completed"`
	Outcome    history.Outcome `json:"outcome,omitempty"`
	CartID     string          `json:"cart_id,omitempty"`
	OrderID    string          `json:"order_id,omitempty"`
	FailedStep string          `json:"failed_step,omitempty"`
	Error      string          `json:"error,omitempty"`
}

func (c *Client) Checkout(ctx context.Context, in CartInput) CheckoutResult {
	var res CheckoutResult
	cartID, err := c.PrepareCart(ctx, in)
	res.CartID = cartID
	if err != nil {
		res.FailedStep = stepOf(err)
		res.Error = err.Error()
		return res
	}
	done, op, err := c.With(Labels{Phase: history.PhaseRace}).CompleteCart(ctx, cartID)
	res.Outcome = op.Outcome
	switch {
	case err != nil:
		res.FailedStep = history.StepComplete
		res.Error = err.Error()
	case op.Outcome == history.OK:
		res.Completed = true
		res.OrderID = done.Order.ID
	default:
		res.FailedStep = history.StepComplete
		res.Error = "complete returned type " + strconv.Quote(done.Type)
		if done.Error != nil {
			res.Error += ": " + done.Error.Message
		}
	}
	return res
}

func stepOf(err error) string {
	var oe *OutcomeError
	if errors.As(err, &oe) {
		return oe.Step
	}
	step, _, _ := strings.Cut(err.Error(), ":")
	return step
}
