import { ENV } from "@/utils/env"

/**
 * Constant type definition
 */
export type ConstantValue = string | number | boolean | null | undefined

/**
 * Constant category enumeration
 */
export enum ConstantCategory {
    SYSTEM = 'system',
    UI = 'ui',
    CONFIG = 'config',
    FEATURE = 'feature'
}

/**
 * Constant storage interface
 */
interface ConstantStore {
    [category: string]: {
        [key: string]: ConstantValue
    }
}

/**
 * Constant storage object
 * Using categories to store different types of constants
 */
const constantStore: ConstantStore = {
    [ConstantCategory.SYSTEM]: {
        VERSION: '1.0.0',
        API_TIMEOUT: Number(ENV.API_TIMEOUT),
        DEBUG_MODE: ENV.isDevelopment,
    },
    [ConstantCategory.UI]: {
        DEFAULT_THEME: 'light',
        // MOBILE_BREAKPOINT: 768,
    },
    [ConstantCategory.CONFIG]: {
        DEFAULT_PAGE_SIZE: 10,
    },
    [ConstantCategory.FEATURE]: {
        QUERY_STALE_TIME: 5 * 60 * 1000,
        DEFAULT_QUERY_RETRY: 1,
        TOAST_DURATION: 2000,
    }
}

/**
 * Function to get a constant
 * @param category Constant category
 * @param key Constant key name
 * @param defaultValue Default value (returned when the constant does not exist)
 * @returns Constant value or default value
 */
export function getConstant<T extends ConstantValue>(
    category: ConstantCategory,
    key: string,
    defaultValue?: T
): T {
    if (
        constantStore[category] &&
        constantStore[category][key] !== undefined
    ) {
        return constantStore[category][key] as T
    }
    return defaultValue as T
}

/**
 * Function to set a constant
 * @param category Constant category
 * @param key Constant key name
 * @param value Value to set
 * @returns Whether the setting was successful
 */
export function setConstant(
    category: ConstantCategory,
    key: string,
    value: ConstantValue
): boolean {
    try {
        // Ensure the category exists
        if (!constantStore[category]) {
            constantStore[category] = {}
        }

        // Set the constant value
        constantStore[category][key] = value
        return true
    } catch (error) {
        console.error(`Failed to set constant [${category}.${key}]:`, error)
        return false
    }
}

/**
 * Check if a constant exists
 * @param category Constant category
 * @param key Constant key name
 * @returns Whether it exists
 */
export function hasConstant(
    category: ConstantCategory,
    key: string
): boolean {
    return (
        constantStore[category] !== undefined &&
        constantStore[category][key] !== undefined
    )
}

/**
 * Get all constants under a category
 * @param category Constant category
 * @returns Constant object
 */
export function getCategoryConstants(
    category: ConstantCategory
): Record<string, ConstantValue> {
    return constantStore[category] || {}
}

/**
 * Batch set constants
 * @param category Constant category
 * @param constants Constant key-value pairs
 * @returns Whether all were set successfully
 */
export function setBatchConstants(
    category: ConstantCategory,
    constants: Record<string, ConstantValue>
): boolean {
    try {
        // Ensure the category exists
        if (!constantStore[category]) {
            constantStore[category] = {}
        }

        // Batch set constants
        Object.entries(constants).forEach(([key, value]) => {
            constantStore[category][key] = value
        })

        return true
    } catch (error) {
        console.error(`Failed to set batch constants for category [${category}]:`, error)
        return false
    }
}