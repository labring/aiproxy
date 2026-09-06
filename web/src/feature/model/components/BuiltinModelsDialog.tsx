import { useCallback, useMemo, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { Check, Search, Sparkles } from "lucide-react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { modelApi } from "@/api/model";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { PriceDisplay } from "@/components/price/PriceDisplay";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { useChannelTypeMetas } from "@/feature/channel/hooks";
import { useChannelBuiltinModels } from "../hooks";
import type { ModelConfig, ModelSaveRequest } from "@/types/model";

interface BuiltinModelsDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  existingModels: ModelConfig[];
  onCreateFromBuiltin: (model: ModelConfig) => void;
  onEditFromBuiltin: (builtinModel: ModelConfig, existingModel: ModelConfig) => void;
}

interface BuiltinModelRow {
  channelType: string;
  model: ModelConfig;
}

const getBuiltinModelRowKey = (row: BuiltinModelRow) =>
  `${row.channelType}:${row.model.model}`;

const getSelectedModelNames = (rows: BuiltinModelRow[], selectedRowKeys: Set<string>) =>
  new Set(
    rows
      .filter((row) => selectedRowKeys.has(getBuiltinModelRowKey(row)))
      .map((row) => row.model.model)
  );

const toModelSaveRequest = (model: ModelConfig): ModelSaveRequest => {
  return {
    config: model.config,
    model: model.model,
    owner: model.owner,
    image_batch_size: model.image_batch_size,
    type: model.type,
    exclude_from_tests: model.exclude_from_tests,
    price: model.price,
    rpm: model.rpm,
    tpm: model.tpm,
    retry_times: model.retry_times,
    retry_budget: model.retry_budget,
    timeout_config: model.timeout_config,
    force_save_detail: model.force_save_detail,
    max_image_generation_count: model.max_image_generation_count,
    max_video_generation_seconds: model.max_video_generation_seconds,
    max_video_generation_count: model.max_video_generation_count,
    allowed_resolutions: model.allowed_resolutions,
    request_body_storage_max_size: model.request_body_storage_max_size,
    response_body_storage_max_size: model.response_body_storage_max_size,
    summary_service_tier: model.summary_service_tier,
    summary_claude_long_context: model.summary_claude_long_context,
    disable_resolution_fuzzy_match: model.disable_resolution_fuzzy_match,
    plugin: model.plugin,
  };
};

export function BuiltinModelsDialog({
  open,
  onOpenChange,
  existingModels,
  onCreateFromBuiltin,
  onEditFromBuiltin,
}: BuiltinModelsDialogProps) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const { data: channelBuiltinModels, isLoading } = useChannelBuiltinModels();
  const { data: channelTypeMetas } = useChannelTypeMetas();
  const [keyword, setKeyword] = useState("");
  const [channelTypeFilter, setChannelTypeFilter] = useState("__all__");
  const [modelTypeFilter, setModelTypeFilter] = useState("__all__");
  const [selectedModels, setSelectedModels] = useState<Set<string>>(new Set());
  const [showExistingModels, setShowExistingModels] = useState(false);
  const [isSaving, setIsSaving] = useState(false);
  const [savingRowKeys, setSavingRowKeys] = useState<Set<string>>(new Set());

  const existingModelSet = useMemo(
    () => new Set(existingModels.map((model) => model.model)),
    [existingModels]
  );

  const existingModelMap = useMemo(
    () => new Map(existingModels.map((model) => [model.model, model])),
    [existingModels]
  );

  const channelTypeOptions = useMemo(() => {
    return Object.keys(channelBuiltinModels || {})
      .map(Number)
      .filter((type) => Number.isFinite(type))
      .sort((a, b) => a - b);
  }, [channelBuiltinModels]);

  const channelScopedRows = useMemo(() => {
    const rows: BuiltinModelRow[] = [];
    for (const [channelType, models] of Object.entries(channelBuiltinModels || {})) {
      if (channelTypeFilter !== "__all__" && channelType !== channelTypeFilter) {
        continue;
      }
      for (const model of models) {
        if (showExistingModels || !existingModelSet.has(model.model)) {
          rows.push({ channelType, model });
        }
      }
    }
    return rows.sort((a, b) => {
      const channelTypeCompare = Number(a.channelType) - Number(b.channelType);
      if (channelTypeCompare !== 0) {
        return channelTypeCompare;
      }
      const modelTypeCompare = a.model.type - b.model.type;
      if (modelTypeCompare !== 0) {
        return modelTypeCompare;
      }
      return a.model.model.localeCompare(b.model.model);
    });
  }, [channelBuiltinModels, channelTypeFilter, existingModelSet, showExistingModels]);

  const allAvailableRows = useMemo(() => {
    const rows: BuiltinModelRow[] = [];
    for (const [channelType, models] of Object.entries(channelBuiltinModels || {})) {
      for (const model of models) {
        if (showExistingModels || !existingModelSet.has(model.model)) {
          rows.push({ channelType, model });
        }
      }
    }
    return rows;
  }, [channelBuiltinModels, existingModelSet, showExistingModels]);

  const modelTypeOptions = useMemo(() => {
    const types = new Set<number>();
    for (const row of channelScopedRows) {
      types.add(row.model.type);
    }
    return [...types].sort((a, b) => a - b);
  }, [channelScopedRows]);

  const filteredRows = useMemo(() => {
    const normalizedKeyword = keyword.trim().toLowerCase();
    return channelScopedRows.filter((row) => {
      if (modelTypeFilter !== "__all__" && String(row.model.type) !== modelTypeFilter) {
        return false;
      }
      if (!normalizedKeyword) {
        return true;
      }
      return row.model.model.toLowerCase().includes(normalizedKeyword);
    });
  }, [channelScopedRows, keyword, modelTypeFilter]);

  const selectedConfigs = useMemo(() => {
    const selected = new Set(selectedModels);
    const selectedModelNames = new Set<string>();
    const configs: ModelConfig[] = [];
    for (const row of allAvailableRows) {
      if (
        !selected.has(getBuiltinModelRowKey(row)) ||
        existingModelSet.has(row.model.model) ||
        selectedModelNames.has(row.model.model)
      ) {
        continue;
      }
      selectedModelNames.add(row.model.model);
      configs.push(row.model);
    }
    return configs;
  }, [allAvailableRows, existingModelSet, selectedModels]);

  const selectedModelNames = useMemo(
    () => getSelectedModelNames(allAvailableRows, selectedModels),
    [allAvailableRows, selectedModels]
  );

  const isRowSelectable = useCallback(
    (row: BuiltinModelRow) => {
      if (existingModelSet.has(row.model.model)) {
        return false;
      }

      const rowKey = getBuiltinModelRowKey(row);
      return !selectedModelNames.has(row.model.model) || selectedModels.has(rowKey);
    },
    [existingModelSet, selectedModelNames, selectedModels]
  );

  const selectableVisibleRows = useMemo(
    () => filteredRows.filter((row) => isRowSelectable(row)),
    [filteredRows, isRowSelectable]
  );

  const selectedVisibleCount = useMemo(
    () =>
      selectableVisibleRows.filter((row) => selectedModels.has(getBuiltinModelRowKey(row)))
        .length,
    [selectableVisibleRows, selectedModels]
  );

  const allVisibleSelected =
    selectableVisibleRows.length > 0 && selectedVisibleCount === selectableVisibleRows.length;

  const toggleModel = (row: BuiltinModelRow) => {
    setSelectedModels((prev) => {
      const next = new Set(prev);
      const rowKey = getBuiltinModelRowKey(row);
      if (next.has(rowKey)) {
        next.delete(rowKey);
      } else {
        if (existingModelSet.has(row.model.model)) {
          return next;
        }
        if (getSelectedModelNames(allAvailableRows, next).has(row.model.model)) {
          return next;
        }
        next.add(rowKey);
      }
      return next;
    });
  };

  const toggleVisibleModels = () => {
    setSelectedModels((prev) => {
      const next = new Set(prev);
      if (allVisibleSelected) {
        for (const row of selectableVisibleRows) {
          next.delete(getBuiltinModelRowKey(row));
        }
      } else {
        const selectedNames = getSelectedModelNames(allAvailableRows, next);
        for (const row of selectableVisibleRows) {
          if (selectedNames.has(row.model.model)) {
            continue;
          }
          next.add(getBuiltinModelRowKey(row));
          selectedNames.add(row.model.model);
        }
      }
      return next;
    });
  };

  const handleSave = async () => {
    if (selectedConfigs.length === 0) {
      return;
    }

    if (selectedConfigs.length === 1) {
      onCreateFromBuiltin(selectedConfigs[0]);
      setSelectedModels(new Set());
      onOpenChange(false);
      return;
    }

    setIsSaving(true);
    try {
      await modelApi.saveModels(selectedConfigs.map(toModelSaveRequest));
      toast.success(t("model.builtin.importSuccess", { count: selectedConfigs.length }));
      await queryClient.invalidateQueries({ queryKey: ["models"] });
      setSelectedModels(new Set());
      onOpenChange(false);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t("model.builtin.importFailed"));
    } finally {
      setIsSaving(false);
    }
  };

  const handleAddBuiltin = async (row: BuiltinModelRow) => {
    const rowKey = getBuiltinModelRowKey(row);
    setSavingRowKeys((prev) => new Set(prev).add(rowKey));

    try {
      await modelApi.saveModels([toModelSaveRequest(row.model)]);
      toast.success(t("model.builtin.importSuccess", { count: 1 }));
      await queryClient.invalidateQueries({ queryKey: ["models"] });
      setSelectedModels((prev) => {
        const next = new Set(prev);
        next.delete(rowKey);
        return next;
      });
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t("model.builtin.importFailed"));
    } finally {
      setSavingRowKeys((prev) => {
        const next = new Set(prev);
        next.delete(rowKey);
        return next;
      });
    }
  };

  const getModelTypeLabel = (type: number) => {
    return t(`modeType.${type}`, { defaultValue: String(type) });
  };

  const getChannelTypeLabel = (type: number) => {
    const typeName = channelTypeMetas?.[String(type)]?.name;
    if (typeName) {
      return typeName;
    }
    return String(type);
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="w-[96vw] sm:max-w-[96vw] xl:w-[1440px] xl:max-w-[1440px] max-h-[88vh] p-0 gap-0 overflow-hidden flex flex-col">
        <DialogHeader className="p-6 pb-4">
          <DialogTitle className="flex items-center gap-2">
            <Sparkles className="h-5 w-5" />
            {t("model.builtin.title")}
          </DialogTitle>
          <DialogDescription>{t("model.builtin.description")}</DialogDescription>
        </DialogHeader>

        <div className="px-6 pb-4 flex flex-col gap-3">
          <div className="flex flex-col gap-2 lg:flex-row">
            <div className="relative flex-1">
              <Search className="absolute left-2.5 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
              <Input
                value={keyword}
                onChange={(event) => setKeyword(event.target.value)}
                placeholder={t("model.builtin.searchPlaceholder")}
                className="pl-8"
              />
            </div>
            <Select
              value={channelTypeFilter}
              onValueChange={(value) => {
                setChannelTypeFilter(value);
                setModelTypeFilter("__all__");
              }}
            >
              <SelectTrigger className="lg:w-56">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="__all__">{t("model.builtin.allChannelTypes")}</SelectItem>
                {channelTypeOptions.map((type) => (
                  <SelectItem key={type} value={String(type)}>
                    {getChannelTypeLabel(type)}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <Select value={modelTypeFilter} onValueChange={setModelTypeFilter}>
              <SelectTrigger className="lg:w-56">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="__all__">{t("model.builtin.allModelTypes")}</SelectItem>
                {modelTypeOptions.map((type) => (
                  <SelectItem key={type} value={String(type)}>
                    {getModelTypeLabel(type)}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <Button
              type="button"
              variant={showExistingModels ? "secondary" : "outline"}
              onClick={() => {
                setShowExistingModels((value) => !value);
                setSelectedModels(new Set());
              }}
              className="lg:w-40"
            >
              {showExistingModels
                ? t("model.builtin.hideExisting")
                : t("model.builtin.showExisting")}
            </Button>
          </div>

          <div className="flex items-center justify-between text-sm text-muted-foreground">
            <span>
              {t("model.builtin.availableCount", {
                count: filteredRows.length,
                total: allAvailableRows.length,
              })}
            </span>
            <span>{t("model.builtin.selectedCount", { count: selectedConfigs.length })}</span>
          </div>
        </div>

        <div className="min-h-0 flex-1 overflow-auto border-y">
          <Table>
            <TableHeader className="sticky top-0 bg-background z-10">
              <TableRow>
                <TableHead className="w-12">
                  <button
                    type="button"
                    className="flex h-5 w-5 items-center justify-center rounded border border-input bg-background disabled:cursor-not-allowed disabled:opacity-50"
                    disabled={selectableVisibleRows.length === 0}
                    onClick={toggleVisibleModels}
                    aria-label={t("model.builtin.selectVisible")}
                  >
                    {allVisibleSelected && <Check className="h-3.5 w-3.5" />}
                  </button>
                </TableHead>
                <TableHead>{t("model.modelName")}</TableHead>
                <TableHead>{t("model.builtin.channelType")}</TableHead>
                <TableHead>{t("model.modelType")}</TableHead>
                <TableHead>{t("model.builtin.status")}</TableHead>
                <TableHead className="w-40" />
                <TableHead>{t("group.price.title")}</TableHead>
                <TableHead>{t("model.configInfo")}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {isLoading ? (
                <TableRow>
                  <TableCell colSpan={8} className="h-32 text-center text-muted-foreground">
                    {t("model.loading")}
                  </TableCell>
                </TableRow>
              ) : filteredRows.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={8} className="h-32 text-center text-muted-foreground">
                    {t("model.builtin.noAvailableModels")}
                  </TableCell>
                </TableRow>
              ) : (
                filteredRows.map((row) => {
                  const model = row.model;
                  const rowKey = getBuiltinModelRowKey(row);
                  const exists = existingModelSet.has(model.model);
                  const existingModel = existingModelMap.get(model.model);
                  const selected = selectedModels.has(rowKey);
                  const disabled = !isRowSelectable(row);
                  const rowSaving = savingRowKeys.has(rowKey);
                  const configBadges = [
                    model.config?.vision && "vision",
                    model.config?.tool_choice && "tool",
                    model.config?.coder && "coder",
                    model.config?.max_context_tokens &&
                      `ctx ${model.config.max_context_tokens.toLocaleString()}`,
                    model.config?.max_output_tokens &&
                      `out ${model.config.max_output_tokens.toLocaleString()}`,
                  ].filter((item): item is string => Boolean(item));

                  return (
                    <TableRow
                      key={rowKey}
                      className={disabled ? "opacity-60" : "cursor-pointer"}
                      data-state={selected ? "selected" : undefined}
                      onClick={() => {
                        if (!disabled) {
                          toggleModel(row);
                        }
                      }}
                    >
                      <TableCell>
                        <button
                          type="button"
                          className="flex h-5 w-5 items-center justify-center rounded border border-input bg-background disabled:cursor-not-allowed disabled:opacity-50"
                          disabled={disabled}
                          onClick={(event) => {
                            event.stopPropagation();
                            toggleModel(row);
                          }}
                          aria-label={`${getChannelTypeLabel(Number(row.channelType))} ${model.model}`}
                        >
                          {selected && <Check className="h-3.5 w-3.5" />}
                        </button>
                      </TableCell>
                      <TableCell className="font-mono font-medium">{model.model}</TableCell>
                      <TableCell>{getChannelTypeLabel(Number(row.channelType))}</TableCell>
                      <TableCell>{getModelTypeLabel(model.type)}</TableCell>
                      <TableCell>
                        <Badge variant={exists ? "secondary" : "outline"} className="text-xs">
                          {exists ? t("model.builtin.added") : t("model.builtin.notAdded")}
                        </Badge>
                      </TableCell>
                      <TableCell>
                        <div className="flex items-center justify-end gap-1.5 whitespace-nowrap">
                          {exists ? (
                            <Button
                              type="button"
                              variant="outline"
                              size="sm"
                              className="h-7 px-2 text-xs"
                              onClick={(event) => {
                                event.stopPropagation();
                                if (existingModel) {
                                  onEditFromBuiltin(model, existingModel);
                                }
                              }}
                            >
                              {t("model.builtin.editFromThis")}
                            </Button>
                          ) : (
                            <>
                              <Button
                                type="button"
                                variant="outline"
                                size="sm"
                                className="h-7 px-2 text-xs"
                                onClick={(event) => {
                                  event.stopPropagation();
                                  onCreateFromBuiltin(model);
                                }}
                              >
                                {t("model.builtin.createFromThis")}
                              </Button>
                              <Button
                                type="button"
                                size="sm"
                                className="h-7 px-2 text-xs"
                                disabled={rowSaving}
                                onClick={(event) => {
                                  event.stopPropagation();
                                  void handleAddBuiltin(row);
                                }}
                              >
                                {rowSaving ? t("model.builtin.importing") : t("model.builtin.add")}
                              </Button>
                            </>
                          )}
                        </div>
                      </TableCell>
                      <TableCell>
                        <PriceDisplay price={model.price} />
                      </TableCell>
                      <TableCell>
                        {configBadges.length === 0 ? (
                          <span className="text-muted-foreground">-</span>
                        ) : (
                          <div className="flex flex-wrap gap-1">
                            {configBadges.map((badge) => (
                              <Badge key={badge} variant="outline" className="text-xs">
                                {badge}
                              </Badge>
                            ))}
                          </div>
                        )}
                      </TableCell>
                    </TableRow>
                  );
                })
              )}
            </TableBody>
          </Table>
        </div>

        <DialogFooter className="p-6 pt-4">
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            {t("common.cancel")}
          </Button>
          <Button onClick={handleSave} disabled={selectedConfigs.length === 0 || isSaving}>
            {isSaving
              ? t("model.builtin.importing")
              : selectedConfigs.length === 1
                ? t("model.builtin.createFromSelected")
                : t("model.builtin.importSelected", { count: selectedConfigs.length })}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
