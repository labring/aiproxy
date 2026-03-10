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

export interface DefaultModelsResponse {
  models: string[];
  mapping: Record<string, string>;
}

export interface AllDefaultModelsResponse {
  models: Record<string, string[]>;
  mapping: Record<string, Record<string, string>>;
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

  // Batch save model configs (for import)
  saveModels: async (configs: ModelCreateRequest[]): Promise<void> => {
    await post("model_configs/", configs);
    return;
  },

  getDefaultModelsByType: async (type: number): Promise<DefaultModelsResponse> => {
    const response = await get<DefaultModelsResponse>(`models/default/${type}`);
    return response;
  },

  getAllDefaultModels: async (): Promise<AllDefaultModelsResponse> => {
    const response = await get<AllDefaultModelsResponse>('models/default');
    return response;
  },
};
