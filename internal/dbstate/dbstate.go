package dbstate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/akallam04/oversold/internal/localguard"
)

var Tables = []string{
	"product_variant",
	"product_variant_inventory_item",
	"inventory_item",
	"inventory_level",
	"reservation_item",
	"order",
	"order_item",
	"order_line_item",
	"order_cart",
	"order_payment_collection",
	"payment_collection",
	"cart_line_item",
	"api_key",
	"publishable_api_key_sales_channel",
}

type DB struct {
	conn *pgx.Conn
}

func Open(ctx context.Context, url string) (*DB, error) {
	if err := localguard.Check(url); err != nil {
		return nil, err
	}
	conn, err := pgx.Connect(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	return &DB{conn: conn}, nil
}

func (db *DB) Close(ctx context.Context) error {
	return db.conn.Close(ctx)
}

func (db *DB) readOnly(ctx context.Context, fn func(pgx.Tx) error) error {
	tx, err := db.conn.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	return fn(tx)
}

func (db *DB) ServerVersion(ctx context.Context) (string, error) {
	var v string
	err := db.conn.QueryRow(ctx, "show server_version").Scan(&v)
	return v, err
}

func (db *DB) PublishableKey(ctx context.Context) (string, error) {
	var tokens []string
	err := db.readOnly(ctx, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			select distinct k.token
			from api_key k
			join publishable_api_key_sales_channel l on l.publishable_key_id = k.id and l.deleted_at is null
			where k.type = 'publishable' and k.revoked_at is null and k.deleted_at is null`)
		if err != nil {
			return err
		}
		tokens, err = pgx.CollectRows(rows, pgx.RowTo[string])
		return err
	})
	if err != nil {
		return "", fmt.Errorf("publishable key: %w", err)
	}
	if len(tokens) != 1 {
		return "", fmt.Errorf("publishable key: found %d live keys linked to a sales channel, want exactly 1", len(tokens))
	}
	return tokens[0], nil
}

type InventoryLevel struct {
	InventoryItemID  string      `json:"inventory_item_id"`
	SKU              string      `json:"sku"`
	LocationID       string      `json:"location_id"`
	StockedQuantity  json.Number `json:"stocked_quantity"`
	ReservedQuantity json.Number `json:"reserved_quantity"`
	UpdatedAt        time.Time   `json:"updated_at"`
}

type Reservation struct {
	ID              string      `json:"id"`
	InventoryItemID string      `json:"inventory_item_id"`
	InventorySKU    string      `json:"inventory_sku"`
	LineItemID      *string     `json:"line_item_id"`
	LineItemSKU     *string     `json:"line_item_sku"`
	LocationID      string      `json:"location_id"`
	Quantity        json.Number `json:"quantity"`
	AllowBackorder  bool        `json:"allow_backorder"`
	CreatedAt       time.Time   `json:"created_at"`
}

type VariantInventory struct {
	VariantID        string      `json:"variant_id"`
	VariantSKU       string      `json:"variant_sku"`
	InventoryItemID  string      `json:"inventory_item_id"`
	InventorySKU     string      `json:"inventory_sku"`
	RequiredQuantity json.Number `json:"required_quantity"`
}

type PaymentCollection struct {
	ID     string      `json:"id"`
	Status string      `json:"status"`
	Amount json.Number `json:"amount"`
}

type OrderLine struct {
	OrderItemID string      `json:"order_item_id"`
	LineItemID  string      `json:"line_item_id"`
	VariantID   *string     `json:"variant_id"`
	VariantSKU  *string     `json:"variant_sku"`
	Quantity    json.Number `json:"quantity"`
}

type Order struct {
	ID                 string              `json:"id"`
	DisplayID          int64               `json:"display_id"`
	Status             string              `json:"status"`
	Version            int32               `json:"version"`
	CreatedAt          time.Time           `json:"created_at"`
	CanceledAt         *time.Time          `json:"canceled_at"`
	Lines              []OrderLine         `json:"lines"`
	PaymentCollections []PaymentCollection `json:"payment_collections"`
}

type CartLine struct {
	ID         string      `json:"id"`
	CartID     string      `json:"cart_id"`
	VariantID  *string     `json:"variant_id"`
	VariantSKU *string     `json:"variant_sku"`
	Quantity   json.Number `json:"quantity"`
}

type CartOrder struct {
	ID        string     `json:"id"`
	CartID    string     `json:"cart_id"`
	OrderID   string     `json:"order_id"`
	DeletedAt *time.Time `json:"deleted_at"`
}

type Snapshot struct {
	TakenAt         time.Time          `json:"taken_at"`
	VariantSKUs     []string           `json:"variant_skus"`
	Variants        []VariantInventory `json:"variants"`
	InventoryLevels []InventoryLevel   `json:"inventory_levels"`
	Reservations    []Reservation      `json:"live_reservations"`
	Orders          []Order            `json:"orders"`
	CartOrders      []CartOrder        `json:"cart_orders"`
	CartLines       []CartLine         `json:"cart_lines"`
}

func (db *DB) Snapshot(ctx context.Context, variantSKUs []string) (*Snapshot, error) {
	if len(variantSKUs) == 0 {
		return nil, errors.New("snapshot: no SKUs")
	}
	s := &Snapshot{VariantSKUs: variantSKUs}
	err := db.readOnly(ctx, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, "select now()").Scan(&s.TakenAt); err != nil {
			return err
		}
		s.TakenAt = s.TakenAt.UTC()

		var err error
		if s.Variants, err = variants(ctx, tx, variantSKUs); err != nil {
			return fmt.Errorf("variants: %w", err)
		}
		if s.InventoryLevels, err = levels(ctx, tx, variantSKUs); err != nil {
			return fmt.Errorf("inventory levels: %w", err)
		}
		if s.Reservations, err = reservations(ctx, tx, variantSKUs); err != nil {
			return fmt.Errorf("reservations: %w", err)
		}
		if s.Orders, err = orders(ctx, tx, variantSKUs); err != nil {
			return fmt.Errorf("orders: %w", err)
		}
		if s.CartOrders, err = cartOrders(ctx, tx, s.Orders); err != nil {
			return fmt.Errorf("cart orders: %w", err)
		}
		if s.CartLines, err = cartLines(ctx, tx, s.CartOrders); err != nil {
			return fmt.Errorf("cart lines: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("snapshot: %w", err)
	}
	return s, nil
}

const itemsForSKUs = `
	select distinct pvi.inventory_item_id
	from product_variant pv
	join product_variant_inventory_item pvi on pvi.variant_id = pv.id and pvi.deleted_at is null
	where pv.sku = any($1) and pv.deleted_at is null`

func variants(ctx context.Context, tx pgx.Tx, skus []string) ([]VariantInventory, error) {
	rows, err := tx.Query(ctx, `
		select pv.id, pv.sku, pvi.inventory_item_id, coalesce(ii.sku, ''), pvi.required_quantity::text
		from product_variant pv
		join product_variant_inventory_item pvi on pvi.variant_id = pv.id and pvi.deleted_at is null
		join inventory_item ii on ii.id = pvi.inventory_item_id
		where pv.sku = any($1) and pv.deleted_at is null
		order by pv.sku, ii.sku`, skus)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (VariantInventory, error) {
		var v VariantInventory
		var req string
		err := r.Scan(&v.VariantID, &v.VariantSKU, &v.InventoryItemID, &v.InventorySKU, &req)
		v.RequiredQuantity = json.Number(req)
		return v, err
	})
}

func levels(ctx context.Context, tx pgx.Tx, skus []string) ([]InventoryLevel, error) {
	rows, err := tx.Query(ctx, `
		select il.inventory_item_id, coalesce(ii.sku, ''), il.location_id,
			il.stocked_quantity::text, il.reserved_quantity::text, il.updated_at
		from inventory_level il
		join inventory_item ii on ii.id = il.inventory_item_id
		where il.deleted_at is null and il.inventory_item_id in (`+itemsForSKUs+`)
		order by ii.sku, il.location_id`, skus)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (InventoryLevel, error) {
		var l InventoryLevel
		var stocked, reserved string
		err := r.Scan(&l.InventoryItemID, &l.SKU, &l.LocationID, &stocked, &reserved, &l.UpdatedAt)
		l.StockedQuantity, l.ReservedQuantity = json.Number(stocked), json.Number(reserved)
		l.UpdatedAt = l.UpdatedAt.UTC()
		return l, err
	})
}

func reservations(ctx context.Context, tx pgx.Tx, skus []string) ([]Reservation, error) {
	rows, err := tx.Query(ctx, `
		select r.id, r.inventory_item_id, coalesce(ii.sku, ''), r.line_item_id,
			coalesce(oli.variant_sku, cli.variant_sku), r.location_id,
			r.quantity::text, coalesce(r.allow_backorder, false), r.created_at
		from reservation_item r
		join inventory_item ii on ii.id = r.inventory_item_id
		left join order_line_item oli on oli.id = r.line_item_id
		left join cart_line_item cli on cli.id = r.line_item_id
		where r.deleted_at is null and r.inventory_item_id in (`+itemsForSKUs+`)
		order by r.created_at, r.id`, skus)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (Reservation, error) {
		var res Reservation
		var qty string
		err := r.Scan(&res.ID, &res.InventoryItemID, &res.InventorySKU, &res.LineItemID,
			&res.LineItemSKU, &res.LocationID, &qty, &res.AllowBackorder, &res.CreatedAt)
		res.Quantity = json.Number(qty)
		res.CreatedAt = res.CreatedAt.UTC()
		return res, err
	})
}

func orders(ctx context.Context, tx pgx.Tx, skus []string) ([]Order, error) {
	rows, err := tx.Query(ctx, `
		select o.id, o.display_id, o.status::text, o.version, o.created_at, o.canceled_at
		from "order" o
		where o.deleted_at is null and exists (
			select 1 from order_item oi
			join order_line_item oli on oli.id = oi.item_id
			where oi.order_id = o.id and oi.version = o.version and oi.deleted_at is null
				and oli.variant_sku = any($1))
		order by o.created_at, o.id`, skus)
	if err != nil {
		return nil, err
	}
	out, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (Order, error) {
		var o Order
		err := r.Scan(&o.ID, &o.DisplayID, &o.Status, &o.Version, &o.CreatedAt, &o.CanceledAt)
		o.CreatedAt = o.CreatedAt.UTC()
		return o, err
	})
	if err != nil {
		return nil, err
	}
	for i := range out {
		lrows, err := tx.Query(ctx, `
			select oi.id, oli.id, oli.variant_id, oli.variant_sku, oi.quantity::text
			from order_item oi
			join order_line_item oli on oli.id = oi.item_id
			where oi.order_id = $1 and oi.version = $2 and oi.deleted_at is null
			order by oli.variant_sku, oli.id`, out[i].ID, out[i].Version)
		if err != nil {
			return nil, err
		}
		out[i].Lines, err = pgx.CollectRows(lrows, func(r pgx.CollectableRow) (OrderLine, error) {
			var l OrderLine
			var qty string
			err := r.Scan(&l.OrderItemID, &l.LineItemID, &l.VariantID, &l.VariantSKU, &qty)
			l.Quantity = json.Number(qty)
			return l, err
		})
		if err != nil {
			return nil, err
		}
		prows, err := tx.Query(ctx, `
			select pc.id, pc.status::text, pc.amount::text
			from order_payment_collection opc
			join payment_collection pc on pc.id = opc.payment_collection_id
			where opc.order_id = $1 and opc.deleted_at is null and pc.deleted_at is null
			order by pc.created_at, pc.id`, out[i].ID)
		if err != nil {
			return nil, err
		}
		out[i].PaymentCollections, err = pgx.CollectRows(prows, func(r pgx.CollectableRow) (PaymentCollection, error) {
			var pc PaymentCollection
			var amount string
			err := r.Scan(&pc.ID, &pc.Status, &amount)
			pc.Amount = json.Number(amount)
			return pc, err
		})
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

func cartOrders(ctx context.Context, tx pgx.Tx, orders []Order) ([]CartOrder, error) {
	ids := make([]string, len(orders))
	for i, o := range orders {
		ids[i] = o.ID
	}
	rows, err := tx.Query(ctx, `
		select id, cart_id, order_id, deleted_at
		from order_cart
		where order_id = any($1)
		order by created_at, id`, ids)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[CartOrder])
}

func cartLines(ctx context.Context, tx pgx.Tx, links []CartOrder) ([]CartLine, error) {
	ids := make([]string, 0, len(links))
	for _, l := range links {
		ids = append(ids, l.CartID)
	}
	rows, err := tx.Query(ctx, `
		select id, cart_id, variant_id, variant_sku, quantity::text
		from cart_line_item
		where cart_id = any($1) and deleted_at is null
		order by cart_id, variant_sku, id`, ids)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (CartLine, error) {
		var l CartLine
		var qty string
		err := r.Scan(&l.ID, &l.CartID, &l.VariantID, &l.VariantSKU, &qty)
		l.Quantity = json.Number(qty)
		return l, err
	})
}
