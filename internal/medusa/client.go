package medusa

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"strings"

	"github.com/akallam04/oversold/internal/history"
	"github.com/akallam04/oversold/internal/localguard"
)

type Labels struct {
	Phase    string
	ActionID string
	Cart     string
	Server   int
}

type Client struct {
	baseURL        string
	publishableKey string
	adminToken     string
	http           *http.Client
	rec            *history.Recorder
	labels         Labels
}

func NewClient(baseURL, publishableKey string, httpClient *http.Client, rec *history.Recorder) (*Client, error) {
	if err := localguard.Check(baseURL); err != nil {
		return nil, err
	}
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	if rec == nil {
		return nil, errors.New("medusa: recorder is required")
	}
	return &Client{
		baseURL:        strings.TrimRight(baseURL, "/"),
		publishableKey: publishableKey,
		http:           httpClient,
		rec:            rec,
		labels:         Labels{Phase: history.PhaseSetup, Server: 1},
	}, nil
}

func (c *Client) With(l Labels) *Client {
	cp := *c
	if l.Phase == "" {
		l.Phase = c.labels.Phase
	}
	if l.Server == 0 {
		l.Server = c.labels.Server
	}
	cp.labels = l
	return &cp
}

func (c *Client) WithAdminToken(token string) *Client {
	cp := *c
	cp.adminToken = token
	return &cp
}

type APIError struct {
	Status  int
	Type    string
	Message string
}

func (e *APIError) Error() string {
	if e.Message == "" {
		return fmt.Sprintf("http %d", e.Status)
	}
	if e.Type == "" {
		return fmt.Sprintf("http %d: %s", e.Status, e.Message)
	}
	return fmt.Sprintf("http %d %s: %s", e.Status, e.Type, e.Message)
}

type OutcomeError struct {
	Step    string
	Outcome history.Outcome
	Err     error
}

func (e *OutcomeError) Error() string {
	return fmt.Sprintf("%s: outcome %s: %v", e.Step, e.Outcome, e.Err)
}

func (e *OutcomeError) Unwrap() error { return e.Err }

func (c *Client) do(ctx context.Context, step, method, path string, in, out any) (history.Operation, error) {
	var reqBody []byte
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return history.Operation{}, fmt.Errorf("%s: encode request: %w", step, err)
		}
		reqBody = b
	}

	var reused *bool
	trace := &httptrace.ClientTrace{GotConn: func(info httptrace.GotConnInfo) {
		r := info.Reused
		reused = &r
	}}
	req, err := http.NewRequestWithContext(httptrace.WithClientTrace(ctx, trace), method, c.baseURL+path, bytes.NewReader(reqBody))
	if err != nil {
		return history.Operation{}, fmt.Errorf("%s: %w", step, err)
	}
	req.Header.Set("Accept", "application/json")
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.publishableKey != "" && strings.HasPrefix(path, "/store/") {
		req.Header.Set("x-publishable-api-key", c.publishableKey)
	}
	if c.adminToken != "" && strings.HasPrefix(path, "/admin/") {
		req.Header.Set("Authorization", "Bearer "+c.adminToken)
	}

	op := history.Operation{
		Phase:    c.labels.Phase,
		Step:     step,
		ActionID: c.labels.ActionID,
		Cart:     c.labels.Cart,
		Server:   c.labels.Server,
		Method:   method,
		Path:     path,
	}
	pending := c.rec.Begin()
	resp, err := c.http.Do(req)
	var respBody []byte
	if err == nil {
		op.Status = resp.StatusCode
		respBody, err = io.ReadAll(resp.Body)
		resp.Body.Close()
	}
	op.ConnReused = reused
	op.Outcome = history.Classify(step, op.Status, respBody, err)

	var callErr error
	switch {
	case err != nil:
		op.Timeout = isTimeout(err)
		callErr = err
	case op.Status < 200 || op.Status > 299:
		apiErr := &APIError{Status: op.Status}
		var body struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		}
		if json.Unmarshal(respBody, &body) == nil {
			apiErr.Type = body.Type
			apiErr.Message = body.Message
		}
		callErr = apiErr
	}
	if callErr != nil {
		op.Error = callErr.Error()
	}
	op = pending.Finish(op, reqBody, respBody)

	if callErr != nil {
		return op, &OutcomeError{Step: step, Outcome: op.Outcome, Err: callErr}
	}
	if out != nil {
		if err := json.Unmarshal(respBody, out); err != nil {
			return op, fmt.Errorf("%s: decode response: %w", step, err)
		}
	}
	return op, nil
}

func (c *Client) mustOK(op history.Operation, err error) error {
	if err != nil {
		return err
	}
	if op.Outcome != history.OK {
		return &OutcomeError{Step: op.Step, Outcome: op.Outcome, Err: errors.New("response was not a definite success")}
	}
	return nil
}

func isTimeout(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var ne net.Error
	return errors.As(err, &ne) && ne.Timeout()
}

type Region struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	CurrencyCode string `json:"currency_code"`
	Countries    []struct {
		Iso2 string `json:"iso_2"`
	} `json:"countries"`
}

type Variant struct {
	ID    string `json:"id"`
	SKU   string `json:"sku"`
	Title string `json:"title"`
}

type Product struct {
	ID       string    `json:"id"`
	Title    string    `json:"title"`
	Variants []Variant `json:"variants"`
}

type LineItem struct {
	ID        string `json:"id"`
	VariantID string `json:"variant_id"`
	Quantity  int    `json:"quantity"`
}

type Cart struct {
	ID    string     `json:"id"`
	Items []LineItem `json:"items"`
}

type ShippingOption struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type PaymentCollection struct {
	ID string `json:"id"`
}

type Order struct {
	ID        string `json:"id"`
	DisplayID int    `json:"display_id"`
}

type CompleteError struct {
	Message string `json:"message"`
	Name    string `json:"name"`
	Type    string `json:"type"`
}

type CompleteResult struct {
	Type  string         `json:"type"`
	Order *Order         `json:"order"`
	Cart  *Cart          `json:"cart"`
	Error *CompleteError `json:"error"`
}

type Address struct {
	FirstName   string `json:"first_name"`
	LastName    string `json:"last_name"`
	Address1    string `json:"address_1"`
	City        string `json:"city"`
	Province    string `json:"province"`
	PostalCode  string `json:"postal_code"`
	CountryCode string `json:"country_code"`
}

type LocationLevel struct {
	ID               string  `json:"id"`
	LocationID       string  `json:"location_id"`
	StockedQuantity  float64 `json:"stocked_quantity"`
	ReservedQuantity float64 `json:"reserved_quantity"`
}

type InventoryItem struct {
	ID             string          `json:"id"`
	SKU            string          `json:"sku"`
	LocationLevels []LocationLevel `json:"location_levels"`
}

func (c *Client) Health(ctx context.Context) error {
	return c.mustOK(c.do(ctx, "warm_up", http.MethodGet, "/health", nil, nil))
}

func (c *Client) ListRegions(ctx context.Context) ([]Region, error) {
	var out struct {
		Regions []Region `json:"regions"`
	}
	err := c.mustOK(c.do(ctx, "list_regions", http.MethodGet, "/store/regions?limit=100", nil, &out))
	return out.Regions, err
}

func (c *Client) ListProducts(ctx context.Context) ([]Product, error) {
	var out struct {
		Products []Product `json:"products"`
	}
	err := c.mustOK(c.do(ctx, "list_products", http.MethodGet, "/store/products?limit=100&fields=id,title,*variants", nil, &out))
	return out.Products, err
}

func (c *Client) CreateCart(ctx context.Context, regionID string) (Cart, error) {
	var out struct {
		Cart Cart `json:"cart"`
	}
	err := c.mustOK(c.do(ctx, "create_cart", http.MethodPost, "/store/carts", map[string]any{"region_id": regionID}, &out))
	return out.Cart, err
}

func (c *Client) AddLineItem(ctx context.Context, cartID, variantID string, quantity int) (Cart, error) {
	var out struct {
		Cart Cart `json:"cart"`
	}
	body := map[string]any{"variant_id": variantID, "quantity": quantity}
	err := c.mustOK(c.do(ctx, "add_line_item", http.MethodPost, "/store/carts/"+url.PathEscape(cartID)+"/line-items", body, &out))
	return out.Cart, err
}

func (c *Client) SetContact(ctx context.Context, cartID, email string, addr Address) (Cart, error) {
	var out struct {
		Cart Cart `json:"cart"`
	}
	body := map[string]any{"email": email, "shipping_address": addr, "billing_address": addr}
	err := c.mustOK(c.do(ctx, "set_contact", http.MethodPost, "/store/carts/"+url.PathEscape(cartID), body, &out))
	return out.Cart, err
}

func (c *Client) ListShippingOptions(ctx context.Context, cartID string) ([]ShippingOption, error) {
	var out struct {
		ShippingOptions []ShippingOption `json:"shipping_options"`
	}
	err := c.mustOK(c.do(ctx, "list_shipping_options", http.MethodGet, "/store/shipping-options?cart_id="+url.QueryEscape(cartID), nil, &out))
	return out.ShippingOptions, err
}

func (c *Client) AddShippingMethod(ctx context.Context, cartID, optionID string) (Cart, error) {
	var out struct {
		Cart Cart `json:"cart"`
	}
	err := c.mustOK(c.do(ctx, "add_shipping_method", http.MethodPost, "/store/carts/"+url.PathEscape(cartID)+"/shipping-methods", map[string]any{"option_id": optionID}, &out))
	return out.Cart, err
}

func (c *Client) CreatePaymentCollection(ctx context.Context, cartID string) (PaymentCollection, error) {
	var out struct {
		PaymentCollection PaymentCollection `json:"payment_collection"`
	}
	err := c.mustOK(c.do(ctx, "create_payment_collection", http.MethodPost, "/store/payment-collections", map[string]any{"cart_id": cartID}, &out))
	return out.PaymentCollection, err
}

func (c *Client) CreatePaymentSession(ctx context.Context, collectionID, providerID string) error {
	return c.mustOK(c.do(ctx, "create_payment_session", http.MethodPost, "/store/payment-collections/"+url.PathEscape(collectionID)+"/payment-sessions", map[string]any{"provider_id": providerID}, nil))
}

func (c *Client) CompleteCart(ctx context.Context, cartID string) (CompleteResult, history.Operation, error) {
	var out CompleteResult
	op, err := c.do(ctx, history.StepComplete, http.MethodPost, "/store/carts/"+url.PathEscape(cartID)+"/complete", nil, &out)
	return out, op, err
}

func (c *Client) AdminLogin(ctx context.Context, email, password string) (string, error) {
	var out struct {
		Token string `json:"token"`
	}
	err := c.mustOK(c.do(ctx, "admin_login", http.MethodPost, "/auth/user/emailpass", map[string]any{"email": email, "password": password}, &out))
	if out.Token != "" {
		c.rec.Masker().Add(out.Token)
	}
	if err == nil && out.Token == "" {
		err = errors.New("admin_login: no token in response")
	}
	return out.Token, err
}

func (c *Client) ListInventoryItems(ctx context.Context, sku string) ([]InventoryItem, error) {
	var out struct {
		InventoryItems []InventoryItem `json:"inventory_items"`
	}
	path := "/admin/inventory-items?sku=" + url.QueryEscape(sku) + "&fields=id,sku,*location_levels"
	err := c.mustOK(c.do(ctx, "list_inventory_items", http.MethodGet, path, nil, &out))
	return out.InventoryItems, err
}

func (c *Client) SetStockedQuantity(ctx context.Context, itemID, locationID string, qty int) error {
	path := "/admin/inventory-items/" + url.PathEscape(itemID) + "/location-levels/" + url.PathEscape(locationID)
	return c.mustOK(c.do(ctx, "set_stocked_quantity", http.MethodPost, path, map[string]any{"stocked_quantity": qty}, nil))
}
