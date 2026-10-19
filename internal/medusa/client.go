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
	"net/url"
	"strings"

	"github.com/akallam04/oversold/internal/localguard"
)

type Client struct {
	baseURL        string
	publishableKey string
	http           *http.Client
	rec            *Recorder
}

func NewClient(baseURL, publishableKey string, httpClient *http.Client, rec *Recorder) (*Client, error) {
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
	}, nil
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

func (c *Client) do(ctx context.Context, step, method, path string, in, out any) error {
	var reqBody []byte
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return fmt.Errorf("%s: encode request: %w", step, err)
		}
		reqBody = b
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(reqBody))
	if err != nil {
		return fmt.Errorf("%s: %w", step, err)
	}
	req.Header.Set("Accept", "application/json")
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.publishableKey != "" {
		req.Header.Set("x-publishable-api-key", c.publishableKey)
	}

	call := Call{Step: step, Method: method, Path: path, RequestBody: rawBody(reqBody)}
	call.Start = c.rec.now()
	resp, err := c.http.Do(req)
	var respBody []byte
	if err == nil {
		call.Status = resp.StatusCode
		respBody, err = io.ReadAll(resp.Body)
		resp.Body.Close()
	}
	call.End = c.rec.now()
	call.ResponseBody = rawBody(respBody)

	if err != nil {
		call.Error = err.Error()
		call.Timeout = isTimeout(err)
		c.rec.add(call)
		return fmt.Errorf("%s: %w", step, err)
	}
	if call.Status < 200 || call.Status > 299 {
		apiErr := &APIError{Status: call.Status}
		var body struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		}
		if json.Unmarshal(respBody, &body) == nil {
			apiErr.Type = body.Type
			apiErr.Message = body.Message
		}
		call.Error = apiErr.Error()
		c.rec.add(call)
		return fmt.Errorf("%s: %w", step, apiErr)
	}
	c.rec.add(call)
	if out != nil {
		if err := json.Unmarshal(respBody, out); err != nil {
			return fmt.Errorf("%s: decode response: %w", step, err)
		}
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

func (c *Client) ListRegions(ctx context.Context) ([]Region, error) {
	var out struct {
		Regions []Region `json:"regions"`
	}
	err := c.do(ctx, "list_regions", http.MethodGet, "/store/regions?limit=100", nil, &out)
	return out.Regions, err
}

func (c *Client) ListProducts(ctx context.Context) ([]Product, error) {
	var out struct {
		Products []Product `json:"products"`
	}
	err := c.do(ctx, "list_products", http.MethodGet, "/store/products?limit=100&fields=id,title,*variants", nil, &out)
	return out.Products, err
}

func (c *Client) CreateCart(ctx context.Context, regionID string) (Cart, error) {
	var out struct {
		Cart Cart `json:"cart"`
	}
	err := c.do(ctx, "create_cart", http.MethodPost, "/store/carts", map[string]any{"region_id": regionID}, &out)
	return out.Cart, err
}

func (c *Client) AddLineItem(ctx context.Context, cartID, variantID string, quantity int) (Cart, error) {
	var out struct {
		Cart Cart `json:"cart"`
	}
	body := map[string]any{"variant_id": variantID, "quantity": quantity}
	err := c.do(ctx, "add_line_item", http.MethodPost, "/store/carts/"+url.PathEscape(cartID)+"/line-items", body, &out)
	return out.Cart, err
}

func (c *Client) SetContact(ctx context.Context, cartID, email string, addr Address) (Cart, error) {
	var out struct {
		Cart Cart `json:"cart"`
	}
	body := map[string]any{"email": email, "shipping_address": addr, "billing_address": addr}
	err := c.do(ctx, "set_contact", http.MethodPost, "/store/carts/"+url.PathEscape(cartID), body, &out)
	return out.Cart, err
}

func (c *Client) ListShippingOptions(ctx context.Context, cartID string) ([]ShippingOption, error) {
	var out struct {
		ShippingOptions []ShippingOption `json:"shipping_options"`
	}
	err := c.do(ctx, "list_shipping_options", http.MethodGet, "/store/shipping-options?cart_id="+url.QueryEscape(cartID), nil, &out)
	return out.ShippingOptions, err
}

func (c *Client) AddShippingMethod(ctx context.Context, cartID, optionID string) (Cart, error) {
	var out struct {
		Cart Cart `json:"cart"`
	}
	err := c.do(ctx, "add_shipping_method", http.MethodPost, "/store/carts/"+url.PathEscape(cartID)+"/shipping-methods", map[string]any{"option_id": optionID}, &out)
	return out.Cart, err
}

func (c *Client) CreatePaymentCollection(ctx context.Context, cartID string) (PaymentCollection, error) {
	var out struct {
		PaymentCollection PaymentCollection `json:"payment_collection"`
	}
	err := c.do(ctx, "create_payment_collection", http.MethodPost, "/store/payment-collections", map[string]any{"cart_id": cartID}, &out)
	return out.PaymentCollection, err
}

func (c *Client) CreatePaymentSession(ctx context.Context, collectionID, providerID string) error {
	return c.do(ctx, "create_payment_session", http.MethodPost, "/store/payment-collections/"+url.PathEscape(collectionID)+"/payment-sessions", map[string]any{"provider_id": providerID}, nil)
}

func (c *Client) CompleteCart(ctx context.Context, cartID string) (CompleteResult, error) {
	var out CompleteResult
	err := c.do(ctx, "complete_cart", http.MethodPost, "/store/carts/"+url.PathEscape(cartID)+"/complete", nil, &out)
	return out, err
}
