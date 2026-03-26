package products

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/rs/zerolog"

	"github.com/droptodrop/droptodrop/pkg/shopify"
)

// ShopProduct represents a product fetched from a Shopify store.
type ShopProduct struct {
	ID          int64              `json:"id"`
	GID         string             `json:"gid"`
	Title       string             `json:"title"`
	Description string             `json:"description"`
	ProductType string             `json:"product_type"`
	Vendor      string             `json:"vendor"`
	Tags        string             `json:"tags"`
	Status      string             `json:"status"`
	Images      []ShopProductImage `json:"images"`
	Variants    []ShopVariant      `json:"variants"`
}

// ShopProductImage is an image from the Shopify store.
type ShopProductImage struct {
	URL     string `json:"url"`
	AltText string `json:"alt_text"`
}

// ShopVariant is a variant from the Shopify store.
type ShopVariant struct {
	ID                int64   `json:"id"`
	GID               string  `json:"gid"`
	Title             string  `json:"title"`
	SKU               string  `json:"sku"`
	Price             string  `json:"price"`
	InventoryQuantity int     `json:"inventory_quantity"`
	Weight            float64 `json:"weight"`
	WeightUnit        string  `json:"weight_unit"`
	RequiresShipping  bool    `json:"requires_shipping"`
}

// FetchShopProducts retrieves products from a shop's Shopify store via GraphQL.
func FetchShopProducts(ctx context.Context, client *shopify.Client, logger zerolog.Logger, cursor string, limit int) ([]ShopProduct, string, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}

	afterClause := ""
	if cursor != "" {
		afterClause = fmt.Sprintf(`, after: "%s"`, cursor)
	}

	query := fmt.Sprintf(`{
		products(first: %d%s, query: "status:active OR status:draft") {
			edges {
				cursor
				node {
					id
					title
					descriptionHtml
					productType
					vendor
					tags
					status
					images(first: 5) {
						edges {
							node {
								url
								altText
							}
						}
					}
					variants(first: 100) {
						edges {
							node {
								id
								title
								sku
								price
								weight
								weightUnit
								requiresShipping
								inventoryItem {
									tracked
								}
							}
						}
					}
				}
			}
			pageInfo {
				hasNextPage
				endCursor
			}
		}
	}`, limit, afterClause)

	var result struct {
		Data struct {
			Products struct {
				Edges []struct {
					Cursor string `json:"cursor"`
					Node   struct {
						ID              string `json:"id"`
						Title           string `json:"title"`
						DescriptionHTML string `json:"descriptionHtml"`
						ProductType     string `json:"productType"`
						Vendor          string `json:"vendor"`
						Tags            []string `json:"tags"`
						Status          string `json:"status"`
						Images          struct {
							Edges []struct {
								Node struct {
									URL     string `json:"url"`
									AltText string `json:"altText"`
								} `json:"node"`
							} `json:"edges"`
						} `json:"images"`
						Variants struct {
							Edges []struct {
								Node struct {
									ID               string  `json:"id"`
									Title            string  `json:"title"`
									SKU              string  `json:"sku"`
									Price            string  `json:"price"`
									Weight           float64 `json:"weight"`
									WeightUnit       string  `json:"weightUnit"`
									RequiresShipping bool    `json:"requiresShipping"`
									InventoryItem    *struct {
										Tracked bool `json:"tracked"`
									} `json:"inventoryItem"`
								} `json:"node"`
							} `json:"edges"`
						} `json:"variants"`
					} `json:"node"`
				} `json:"edges"`
				PageInfo struct {
					HasNextPage bool   `json:"hasNextPage"`
					EndCursor   string `json:"endCursor"`
				} `json:"pageInfo"`
			} `json:"products"`
		} `json:"data"`
	}

	if err := client.GraphQL(ctx, query, nil, &result); err != nil {
		return nil, "", fmt.Errorf("fetch products: %w", err)
	}

	logger.Info().Int("product_count", len(result.Data.Products.Edges)).Msg("fetched products from Shopify")

	products := make([]ShopProduct, 0, len(result.Data.Products.Edges))
	for _, edge := range result.Data.Products.Edges {
		node := edge.Node

		numericID, _ := shopify.ParseGID(node.ID)

		var images []ShopProductImage
		for _, imgEdge := range node.Images.Edges {
			images = append(images, ShopProductImage{
				URL:     imgEdge.Node.URL,
				AltText: imgEdge.Node.AltText,
			})
		}

		var variants []ShopVariant
		for _, vEdge := range node.Variants.Edges {
			vn := vEdge.Node
			variantID, _ := shopify.ParseGID(vn.ID)
			variants = append(variants, ShopVariant{
				ID:               variantID,
				GID:              vn.ID,
				Title:            vn.Title,
				SKU:              vn.SKU,
				Price:            vn.Price,
				Weight:           vn.Weight,
				WeightUnit:       vn.WeightUnit,
				RequiresShipping: vn.RequiresShipping,
			})
		}

		products = append(products, ShopProduct{
			ID:          numericID,
			GID:         node.ID,
			Title:       node.Title,
			Description: node.DescriptionHTML,
			ProductType: node.ProductType,
			Vendor:      node.Vendor,
			Tags:        strings.Join(node.Tags, ", "),
			Status:      node.Status,
			Images:      images,
			Variants:    variants,
		})
	}

	nextCursor := ""
	if result.Data.Products.PageInfo.HasNextPage {
		nextCursor = result.Data.Products.PageInfo.EndCursor
	}

	return products, nextCursor, nil
}

// ParsePrice parses a price string to float64.
func ParsePrice(s string) float64 {
	f, _ := strconv.ParseFloat(s, 64)
	return f
}
