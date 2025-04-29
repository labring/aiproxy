declare global {
    type ApiError = import('../api').ApiError
}

// this is required to make the file a module
export { }