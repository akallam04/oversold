"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
exports.confirmInventoryStep = exports.aggregateConfirmInventoryItems = exports.confirmInventoryStepId = void 0;
const utils_1 = require("@medusajs/framework/utils");
const workflows_sdk_1 = require("@medusajs/framework/workflows-sdk");
exports.confirmInventoryStepId = "confirm-inventory-step";
const aggregateConfirmInventoryItems = (items) => {
    const requests = new Map();
    for (const item of items) {
        if (item.allow_backorder) {
            continue;
        }
        const quantity = utils_1.MathBN.mult(item.quantity, item.required_quantity);
        const request = requests.get(item.inventory_item_id);
        if (request) {
            request.quantity = utils_1.MathBN.add(request.quantity, quantity);
            request.location_ids = Array.from(new Set([...request.location_ids, ...item.location_ids]));
            continue;
        }
        requests.set(item.inventory_item_id, {
            inventory_item_id: item.inventory_item_id,
            location_ids: [...item.location_ids],
            quantity,
        });
    }
    return Array.from(requests.values());
};
exports.aggregateConfirmInventoryItems = aggregateConfirmInventoryItems;
/**
 * This step validates that items in the cart have sufficient inventory quantity.
 * If an item doesn't have sufficient inventory, an error is thrown.
 *
 * @example
 * confirmInventoryStep({
 *   items: [
 *     {
 *       inventory_item_id: "iitem_123",
 *       required_quantity: 1,
 *       allow_backorder: false,
 *       quantity: 1,
 *       location_ids: ["sloc_123"]
 *     }
 *   ]
 * })
 */
exports.confirmInventoryStep = (0, workflows_sdk_1.createStep)(exports.confirmInventoryStepId, async (data, { container }) => {
    if (!data.items?.length) {
        return new workflows_sdk_1.StepResponse([], []);
    }
    const inventoryService = container.resolve(utils_1.Modules.INVENTORY);
    const promises = (0, exports.aggregateConfirmInventoryItems)(data.items).map((item) => inventoryService.confirmInventory(item.inventory_item_id, item.location_ids, item.quantity));
    const inventoryCoverage = await (0, utils_1.promiseAll)(promises);
    if (inventoryCoverage.some((hasCoverage) => !hasCoverage)) {
        throw new utils_1.MedusaError(utils_1.MedusaError.Types.NOT_ALLOWED, `Some variant does not have the required inventory`, utils_1.MedusaError.Codes.INSUFFICIENT_INVENTORY);
    }
    return new workflows_sdk_1.StepResponse(null);
});
//# sourceMappingURL=confirm-inventory.js.map