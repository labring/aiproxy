// src/feature/model/hooks.ts
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { modelApi } from '@/api/model'
import { useState } from 'react'
import { ModelCreateRequest } from '@/types/model'
import { toast } from 'sonner'

// Get all models
export const useModels = () => {
    const query = useQuery({
        queryKey: ['models'],
        queryFn: modelApi.getModels,
    })

    return {
        ...query,
    }
}

// Get a specific model
export const useModel = (model: string) => {
    const query = useQuery({
        queryKey: ['model', model],
        queryFn: () => modelApi.getModel(model),
        enabled: !!model,
    })

    return {
        ...query,
    }
}

// Create a new model
export const useCreateModel = () => {
    const queryClient = useQueryClient()
    const [error, setError] = useState<ApiError | null>(null)

    const mutation = useMutation({
        mutationFn: (data: ModelCreateRequest) => {
            return modelApi.createModel(data)
        },
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ['models'] })
            setError(null)
        },
        onError: (err: ApiError) => {
            setError(err)
            toast.error(err.message)
        },
    })

    return {
        createModel: mutation.mutate,
        isLoading: mutation.isPending,
        error,
        clearError: () => setError(null),
    }
}

// Delete a model
export const useDeleteModel = () => {
    const queryClient = useQueryClient()
    const [error, setError] = useState<ApiError | null>(null)

    const mutation = useMutation({
        mutationFn: (model: string) => {
            return modelApi.deleteModel(model)
        },
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ['models'] })
            setError(null)
        },
        onError: (err: ApiError) => {
            setError(err)
            toast.error(err.message)
        },
    })

    return {
        deleteModel: mutation.mutate,
        isLoading: mutation.isPending,
        error,
        clearError: () => setError(null),
    }
}