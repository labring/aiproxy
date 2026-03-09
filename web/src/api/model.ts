// src/api/model.ts
import { get, post, del } from "./index";
import { ModelConfig, ModelCreateRequest } from "@/types/model";

// Define the type for model sets response
export interface ModelSetsResponse {
  [modelName: string]: {
    [setName: string]: Array<{
      id: number;
      type: number;
      name: string;
      priority: number;
      weight: number; // 权重百分比 (0-100)
    }>;
  };
}

export const modelApi = {
  getModels: async (): Promise<ModelConfig[]> => {
    const response = await get<ModelConfig[]>("model_configs/all");
    return response;
  },

  getModel: async (model: string): Promise<ModelConfig> => {
    const response = await get<ModelConfig>(`model_config/${model}`);
    return response;
  },

  getModelSets: async () => {
    const response = await get<ModelSetsResponse>("models/sets");
    return response;
  },

  createModel: async (
    model: string,
    data: Omit<ModelCreateRequest, "model">
  ): Promise<void> => {
    await post(`model_config/${model}`, data);
    return;
  },

  updateModel: async (
    model: string,
    data: Omit<ModelCreateRequest, "model">
  ): Promise<void> => {
    await post(`model_config/${model}`, data);
    return;
  },

  deleteModel: async (model: string): Promise<void> => {
    await del(`model_config/${model}`);
    return;
  },
};
