package shopify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/rs/zerolog"
)

const (
	apiVersion     = "2024-10"
	graphqlPath    = "/admin/api/%s/graphql.json"
	restPath       = "/admin/api/%s"
)

// Client is a Shopify API client for a specific shop.
type Client struct {
	shopDomain  string
	accessToken string
	httpClient  *http.Client
	logger      zerolog.Logger
}

// NewClient creates a new Shopify API client.
func NewClient(shopDomain, accessToken string, logger zerolog.Logger) *Client {
	return &Client{
		shopDomain:  shopDomain,
		accessToken: accessToken,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: logger,
	}
}

// GraphQL executes a GraphQL query against the Shopify Admin API.
func (c *Client) GraphQL(ctx context.Context, query string, variables map[string]interface{}, result interface{}) error {
	body := map[string]interface{}{
		"query":     query,
		"variables": variables,
	}

	data, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal graphql body: %w", err)
	}

	url := fmt.Sprintf("https://%s"+graphqlPath, c.shopDomain, apiVersion)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Shopify-Access-Token", c.accessToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("shopify API error: status=%d body=%s", resp.StatusCode, string(respBody))
	}

	if result != nil {
		if err := json.Unmarshal(respBody, result); err != nil {
			return fmt.Errorf("unmarshal response: %w", err)
		}
	}

	return nil
}

// REST executes a REST API call against the Shopify Admin API.
func (c *Client) REST(ctx context.Context, method, endpoint string, body interface{}, result interface{}) error {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal body: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	url := fmt.Sprintf("https://%s"+restPath+"/%s", c.shopDomain, apiVersion, endpoint)
	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Shopify-Access-Token", c.accessToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("shopify REST error: status=%d body=%s", resp.StatusCode, string(respBody))
	}

	if result != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, result); err != nil {
			return fmt.Errorf("unmarshal response: %w", err)
		}
	}

	return nil
}

// RegisterWebhook registers a webhook subscription via GraphQL.
func (c *Client) RegisterWebhook(ctx context.Context, topic, callbackURL string) error {
	query := `mutation webhookSubscriptionCreate($topic: WebhookSubscriptionTopic!, $webhookSubscription: WebhookSubscriptionInput!) {
		webhookSubscriptionCreate(topic: $topic, webhookSubscription: $webhookSubscription) {
			webhookSubscription { id }
			userErrors { field message }
		}
	}`

	variables := map[string]interface{}{
		"topic": topic,
		"webhookSubscription": map[string]interface{}{
			"callbackUrl": callbackURL,
			"format":      "JSON",
		},
	}

	var result struct {
		Data struct {
			WebhookSubscriptionCreate struct {
				UserErrors []struct {
					Field   []string `json:"field"`
					Message string   `json:"message"`
				} `json:"userErrors"`
			} `json:"webhookSubscriptionCreate"`
		} `json:"data"`
	}

	if err := c.GraphQL(ctx, query, variables, &result); err != nil {
		return err
	}

	if len(result.Data.WebhookSubscriptionCreate.UserErrors) > 0 {
		return fmt.Errorf("webhook registration error: %s",
			result.Data.WebhookSubscriptionCreate.UserErrors[0].Message)
	}

	return nil
}

// ---- Typed response structs ----

// UserError represents a Shopify GraphQL user error.
type UserError struct {
	Field   []string `json:"field"`
	Message string   `json:"message"`
}

// ProductVariantNode is a variant inside a GraphQL product response.
type ProductVariantNode struct {
	ID                string `json:"id"` // GID like "gid://shopify/ProductVariant/123"
	Title             string `json:"title"`
	SKU               string `json:"sku"`
	Price             string `json:"price"`
	InventoryQuantity int    `json:"inventoryQuantity"`
}

// ProductNode is a product inside a GraphQL response.
type ProductNode struct {
	ID       string `json:"id"` // GID like "gid://shopify/Product/123"
	Title    string `json:"title"`
	Variants struct {
		Edges []struct {
			Node ProductVariantNode `json:"node"`
		} `json:"edges"`
	} `json:"variants"`
}

// CreateProductResponse is the typed response from productCreate.
type CreateProductResponse struct {
	Data struct {
		ProductCreate struct {
			Product    *ProductNode `json:"product"`
			UserErrors []UserError  `json:"userErrors"`
		} `json:"productCreate"`
	} `json:"data"`
}

// FulfillmentNode is a fulfillment inside a GraphQL response.
type FulfillmentNode struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

// CreateFulfillmentResponse is the typed response from fulfillmentCreateV2.
type CreateFulfillmentResponse struct {
	Data struct {
		FulfillmentCreateV2 struct {
			Fulfillment *FulfillmentNode `json:"fulfillment"`
			UserErrors  []UserError      `json:"userErrors"`
		} `json:"fulfillmentCreateV2"`
	} `json:"data"`
}

// FulfillmentOrderNode represents a fulfillment order from Shopify.
type FulfillmentOrderNode struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

// ParseGID extracts the numeric ID from a Shopify GID string.
// e.g. "gid://shopify/Product/123" → 123
func ParseGID(gid string) (int64, error) {
	// Find the last "/" and parse the number after it
	for i := len(gid) - 1; i >= 0; i-- {
		if gid[i] == '/' {
			var id int64
			_, err := fmt.Sscanf(gid[i+1:], "%d", &id)
			return id, err
		}
	}
	return 0, fmt.Errorf("invalid GID: %s", gid)
}

// ---- Product operations ----

// CreateProduct creates a product in the shop using GraphQL and returns the typed response.
func (c *Client) CreateProduct(ctx context.Context, input map[string]interface{}) (*CreateProductResponse, error) {
	query := `mutation productCreate($input: ProductInput!) {
		productCreate(input: $input) {
			product {
				id
				title
				variants(first: 100) {
					edges {
						node {
							id
							title
							sku
							price
							inventoryQuantity
						}
					}
				}
			}
			userErrors { field message }
		}
	}`

	variables := map[string]interface{}{"input": input}

	var result CreateProductResponse
	if err := c.GraphQL(ctx, query, variables, &result); err != nil {
		return nil, err
	}

	if len(result.Data.ProductCreate.UserErrors) > 0 {
		return nil, fmt.Errorf("productCreate error: %s (field: %v)",
			result.Data.ProductCreate.UserErrors[0].Message,
			result.Data.ProductCreate.UserErrors[0].Field)
	}

	if result.Data.ProductCreate.Product == nil {
		return nil, fmt.Errorf("productCreate returned nil product")
	}

	return &result, nil
}

// ---- Fulfillment operations ----

// GetFulfillmentOrders returns the fulfillment orders for a given Shopify order.
func (c *Client) GetFulfillmentOrders(ctx context.Context, orderID int64) ([]FulfillmentOrderNode, error) {
	query := `query getFulfillmentOrders($orderId: ID!) {
		order(id: $orderId) {
			fulfillmentOrders(first: 10) {
				edges {
					node {
						id
						status
					}
				}
			}
		}
	}`

	orderGID := fmt.Sprintf("gid://shopify/Order/%d", orderID)
	variables := map[string]interface{}{"orderId": orderGID}

	var result struct {
		Data struct {
			Order struct {
				FulfillmentOrders struct {
					Edges []struct {
						Node FulfillmentOrderNode `json:"node"`
					} `json:"edges"`
				} `json:"fulfillmentOrders"`
			} `json:"order"`
		} `json:"data"`
	}

	if err := c.GraphQL(ctx, query, variables, &result); err != nil {
		return nil, err
	}

	var orders []FulfillmentOrderNode
	for _, edge := range result.Data.Order.FulfillmentOrders.Edges {
		orders = append(orders, edge.Node)
	}
	return orders, nil
}

// CreateFulfillment creates a fulfillment for a fulfillment order with tracking info.
func (c *Client) CreateFulfillment(ctx context.Context, fulfillmentOrderID string, trackingNumber, trackingURL, trackingCompany string) (*FulfillmentNode, error) {
	query := `mutation fulfillmentCreateV2($fulfillment: FulfillmentV2Input!) {
		fulfillmentCreateV2(fulfillment: $fulfillment) {
			fulfillment { id status }
			userErrors { field message }
		}
	}`

	variables := map[string]interface{}{
		"fulfillment": map[string]interface{}{
			"lineItemsByFulfillmentOrder": []map[string]interface{}{
				{"fulfillmentOrderId": fulfillmentOrderID},
			},
			"trackingInfo": map[string]interface{}{
				"number":  trackingNumber,
				"url":     trackingURL,
				"company": trackingCompany,
			},
			"notifyCustomer": true,
		},
	}

	var result CreateFulfillmentResponse
	if err := c.GraphQL(ctx, query, variables, &result); err != nil {
		return nil, err
	}

	if len(result.Data.FulfillmentCreateV2.UserErrors) > 0 {
		return nil, fmt.Errorf("fulfillmentCreateV2 error: %s (field: %v)",
			result.Data.FulfillmentCreateV2.UserErrors[0].Message,
			result.Data.FulfillmentCreateV2.UserErrors[0].Field)
	}

	return result.Data.FulfillmentCreateV2.Fulfillment, nil
}

// ---- Product read ----

// GetProduct fetches a product by GID.
func (c *Client) GetProduct(ctx context.Context, productGID string) (*ProductNode, error) {
	query := `query getProduct($id: ID!) {
		product(id: $id) {
			id title
			variants(first: 100) {
				edges {
					node {
						id title sku price
						inventoryQuantity
					}
				}
			}
		}
	}`

	variables := map[string]interface{}{"id": productGID}

	var result struct {
		Data struct {
			Product *ProductNode `json:"product"`
		} `json:"data"`
	}
	if err := c.GraphQL(ctx, query, variables, &result); err != nil {
		return nil, err
	}
	if result.Data.Product == nil {
		return nil, fmt.Errorf("product not found: %s", productGID)
	}
	return result.Data.Product, nil
}

// ---- Inventory operations ----

// SetInventoryQuantity sets the available inventory for a variant at a location.
// Uses the inventorySetQuantities mutation (2024-10 API).
func (c *Client) SetInventoryQuantity(ctx context.Context, inventoryItemID int64, locationID int64, quantity int) error {
	query := `mutation inventorySetQuantities($input: InventorySetQuantitiesInput!) {
		inventorySetQuantities(input: $input) {
			inventoryAdjustmentGroup {
				reason
			}
			userErrors { field message }
		}
	}`

	itemGID := fmt.Sprintf("gid://shopify/InventoryItem/%d", inventoryItemID)
	locationGID := fmt.Sprintf("gid://shopify/Location/%d", locationID)

	variables := map[string]interface{}{
		"input": map[string]interface{}{
			"name":   "available",
			"reason": "correction",
			"quantities": []map[string]interface{}{
				{
					"inventoryItemId": itemGID,
					"locationId":      locationGID,
					"quantity":        quantity,
				},
			},
		},
	}

	var result struct {
		Data struct {
			InventorySetQuantities struct {
				UserErrors []UserError `json:"userErrors"`
			} `json:"inventorySetQuantities"`
		} `json:"data"`
	}

	if err := c.GraphQL(ctx, query, variables, &result); err != nil {
		return err
	}

	if len(result.Data.InventorySetQuantities.UserErrors) > 0 {
		return fmt.Errorf("inventorySetQuantities error: %s",
			result.Data.InventorySetQuantities.UserErrors[0].Message)
	}

	return nil
}

// GetShopLocations retrieves the shop's locations (needed for inventory operations).
func (c *Client) GetShopLocations(ctx context.Context) ([]LocationNode, error) {
	query := `{
		locations(first: 10) {
			edges {
				node {
					id
					name
					isActive
					isPrimary
				}
			}
		}
	}`

	var result struct {
		Data struct {
			Locations struct {
				Edges []struct {
					Node LocationNode `json:"node"`
				} `json:"edges"`
			} `json:"locations"`
		} `json:"data"`
	}

	if err := c.GraphQL(ctx, query, nil, &result); err != nil {
		return nil, err
	}

	var locations []LocationNode
	for _, edge := range result.Data.Locations.Edges {
		locations = append(locations, edge.Node)
	}
	return locations, nil
}

// LocationNode represents a Shopify location.
type LocationNode struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	IsActive  bool   `json:"isActive"`
	IsPrimary bool   `json:"isPrimary"`
}

// GetVariantInventoryItem fetches the inventory item ID for a product variant.
func (c *Client) GetVariantInventoryItem(ctx context.Context, variantID int64) (int64, error) {
	query := `query getVariant($id: ID!) {
		productVariant(id: $id) {
			inventoryItem {
				id
			}
		}
	}`

	gid := fmt.Sprintf("gid://shopify/ProductVariant/%d", variantID)
	variables := map[string]interface{}{"id": gid}

	var result struct {
		Data struct {
			ProductVariant struct {
				InventoryItem struct {
					ID string `json:"id"`
				} `json:"inventoryItem"`
			} `json:"productVariant"`
		} `json:"data"`
	}

	if err := c.GraphQL(ctx, query, variables, &result); err != nil {
		return 0, err
	}

	return ParseGID(result.Data.ProductVariant.InventoryItem.ID)
}
