import { ExecArgs } from "@medusajs/framework/types"
import {
  ContainerRegistrationKeys,
  Modules,
  ProductStatus,
} from "@medusajs/framework/utils"
import {
  createApiKeysWorkflow,
  createInventoryItemsWorkflow,
  createProductsWorkflow,
  createRegionsWorkflow,
  createSalesChannelsWorkflow,
  createShippingOptionsWorkflow,
  createShippingProfilesWorkflow,
  createStockLocationsWorkflow,
  createTaxRegionsWorkflow,
  createUsersWorkflow,
  linkSalesChannelsToApiKeyWorkflow,
  linkSalesChannelsToStockLocationWorkflow,
  updateStoresWorkflow,
} from "@medusajs/medusa/core-flows"

const SKUS = ["OVS-PIECE", "OVS-TWOPACK", "OVS-SOLO"]

async function ensureAdmin(container: ExecArgs["container"]) {
  const logger = container.resolve(ContainerRegistrationKeys.LOGGER)
  const email = process.env.MEDUSA_ADMIN_EMAIL
  const password = process.env.MEDUSA_ADMIN_PASSWORD
  if (!email || !password) {
    throw new Error("oversold seed: MEDUSA_ADMIN_EMAIL and MEDUSA_ADMIN_PASSWORD must be set")
  }

  const users = container.resolve(Modules.USER)
  const [existing] = await users.listUsers({ email })
  if (existing) {
    logger.info("oversold seed: admin user already exists")
    return
  }

  const {
    result: [user],
  } = await createUsersWorkflow(container).run({
    input: { users: [{ email }] },
  })

  const auth = container.resolve(Modules.AUTH)
  const { authIdentity, error } = await auth.register("emailpass", {
    body: { email, password },
  })
  if (error || !authIdentity) {
    throw new Error(`oversold seed: could not register admin identity: ${error}`)
  }
  await auth.updateAuthIdentities({
    id: authIdentity.id,
    app_metadata: { user_id: user.id },
  })
  logger.info("oversold seed: admin user created")
}

export default async function seed({ container }: ExecArgs) {
  await ensureAdmin(container)

  const logger = container.resolve(ContainerRegistrationKeys.LOGGER)
  const link = container.resolve(ContainerRegistrationKeys.LINK)
  const query = container.resolve(ContainerRegistrationKeys.QUERY)
  const fulfillment = container.resolve(Modules.FULFILLMENT)
  const salesChannels = container.resolve(Modules.SALES_CHANNEL)
  const stores = container.resolve(Modules.STORE)

  const { data: existing } = await query.graph({
    entity: "product_variant",
    fields: ["id", "sku"],
    filters: { sku: SKUS },
  })
  if (existing.length === SKUS.length) {
    logger.info("oversold seed: SKUs already exist, nothing to do")
    return
  }
  if (existing.length > 0) {
    throw new Error(
      `oversold seed: found only some SKUs (${existing.map((v) => v.sku).join(", ")}), refusing to seed a partial store`
    )
  }

  const [store] = await stores.listStores()

  let [channel] = await salesChannels.listSalesChannels({
    name: "Default Sales Channel",
  })
  if (!channel) {
    const { result } = await createSalesChannelsWorkflow(container).run({
      input: { salesChannelsData: [{ name: "Default Sales Channel" }] },
    })
    channel = result[0]
  }

  const {
    result: [region],
  } = await createRegionsWorkflow(container).run({
    input: {
      regions: [
        {
          name: "US",
          currency_code: "usd",
          countries: ["us"],
          payment_providers: ["pp_system_default"],
        },
      ],
    },
  })

  await updateStoresWorkflow(container).run({
    input: {
      selector: { id: store.id },
      update: {
        supported_currencies: [{ currency_code: "usd", is_default: true }],
        default_sales_channel_id: channel.id,
        default_region_id: region.id,
      },
    },
  })

  await createTaxRegionsWorkflow(container).run({
    input: [{ country_code: "us", provider_id: "tp_system" }],
  })

  const {
    result: [location],
  } = await createStockLocationsWorkflow(container).run({
    input: {
      locations: [
        {
          name: "US Warehouse",
          address: { city: "Austin", country_code: "US", address_1: "" },
        },
      ],
    },
  })

  await updateStoresWorkflow(container).run({
    input: {
      selector: { id: store.id },
      update: { default_location_id: location.id },
    },
  })

  await link.create({
    [Modules.STOCK_LOCATION]: { stock_location_id: location.id },
    [Modules.FULFILLMENT]: { fulfillment_provider_id: "manual_manual" },
  })

  let [profile] = await fulfillment.listShippingProfiles({ type: "default" })
  if (!profile) {
    const { result } = await createShippingProfilesWorkflow(container).run({
      input: {
        data: [{ name: "Default Shipping Profile", type: "default" }],
      },
    })
    profile = result[0]
  }

  const fulfillmentSet = await fulfillment.createFulfillmentSets({
    name: "US Warehouse delivery",
    type: "shipping",
    service_zones: [
      { name: "US", geo_zones: [{ country_code: "us", type: "country" }] },
    ],
  })

  await link.create({
    [Modules.STOCK_LOCATION]: { stock_location_id: location.id },
    [Modules.FULFILLMENT]: { fulfillment_set_id: fulfillmentSet.id },
  })

  await createShippingOptionsWorkflow(container).run({
    input: [
      {
        name: "Standard Shipping",
        price_type: "flat",
        provider_id: "manual_manual",
        service_zone_id: fulfillmentSet.service_zones[0].id,
        shipping_profile_id: profile.id,
        type: {
          label: "Standard",
          description: "Flat rate US shipping.",
          code: "standard",
        },
        prices: [
          { currency_code: "usd", amount: 5 },
          { region_id: region.id, amount: 5 },
        ],
        rules: [
          { attribute: "enabled_in_store", value: "true", operator: "eq" },
          { attribute: "is_return", value: "false", operator: "eq" },
        ],
      },
    ],
  })

  await linkSalesChannelsToStockLocationWorkflow(container).run({
    input: { id: location.id, add: [channel.id] },
  })

  const { data: keys } = await query.graph({
    entity: "api_key",
    fields: ["id"],
    filters: { type: "publishable" },
  })
  let apiKey: { id: string } | undefined = keys[0]
  if (!apiKey) {
    const { result } = await createApiKeysWorkflow(container).run({
      input: {
        api_keys: [{ title: "Oversold", type: "publishable", created_by: "" }],
      },
    })
    apiKey = result[0]
  }

  await linkSalesChannelsToApiKeyWorkflow(container).run({
    input: { id: apiKey.id, add: [channel.id] },
  })

  const { result: items } = await createInventoryItemsWorkflow(container).run({
    input: {
      items: [
        {
          sku: "WIDGET",
          title: "Widget",
          location_levels: [{ location_id: location.id, stocked_quantity: 10 }],
        },
        {
          sku: "SOLO",
          title: "Solo",
          location_levels: [{ location_id: location.id, stocked_quantity: 1 }],
        },
      ],
    },
  })
  const widget = items.find((i) => i.sku === "WIDGET")!
  const solo = items.find((i) => i.sku === "SOLO")!

  await createProductsWorkflow(container).run({
    input: {
      products: [
        {
          title: "Widget",
          handle: "widget",
          status: ProductStatus.PUBLISHED,
          shipping_profile_id: profile.id,
          options: [{ title: "Pack", values: ["Piece", "Two-pack"] }],
          variants: [
            {
              title: "Piece",
              sku: "OVS-PIECE",
              manage_inventory: true,
              options: { Pack: "Piece" },
              prices: [{ currency_code: "usd", amount: 10 }],
              inventory_items: [
                { inventory_item_id: widget.id, required_quantity: 1 },
              ],
            },
            {
              title: "Two-pack",
              sku: "OVS-TWOPACK",
              manage_inventory: true,
              options: { Pack: "Two-pack" },
              prices: [{ currency_code: "usd", amount: 18 }],
              inventory_items: [
                { inventory_item_id: widget.id, required_quantity: 2 },
              ],
            },
          ],
          sales_channels: [{ id: channel.id }],
        },
        {
          title: "Solo",
          handle: "solo",
          status: ProductStatus.PUBLISHED,
          shipping_profile_id: profile.id,
          options: [{ title: "Default", values: ["Default"] }],
          variants: [
            {
              title: "Default",
              sku: "OVS-SOLO",
              manage_inventory: true,
              options: { Default: "Default" },
              prices: [{ currency_code: "usd", amount: 25 }],
              inventory_items: [
                { inventory_item_id: solo.id, required_quantity: 1 },
              ],
            },
          ],
          sales_channels: [{ id: channel.id }],
        },
      ],
    },
  })

  logger.info("oversold seed: done")
}
