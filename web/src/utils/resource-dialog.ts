import { ApiError } from '@/api/index'
import { toast } from 'sonner'

const isNotFoundError = (error: unknown) => error instanceof ApiError && error.code === 404

export const openResourceDialog = async <T>({
    fetcher,
    onSuccess,
    onNotFound,
    onError,
}: {
    fetcher: () => Promise<T>
    onSuccess: (resource: T) => void
    onNotFound: () => void
    onError: (error: unknown) => void
}) => {
    try {
        const resource = await fetcher()
        onSuccess(resource)
    } catch (error) {
        if (isNotFoundError(error)) {
            onNotFound()
            return
        }
        onError(error)
    }
}

export const showDeletedResourceToast = (message: string) => {
    toast.error(message)
}
