import type { Asset } from "@/stores/use-asset-store";

export type AssetLibraryScope = "works" | "materials";

export function getAssetLibraryScope(asset: Asset): AssetLibraryScope {
    const metadataSource = typeof asset.metadata?.source === "string" ? asset.metadata.source : "";
    const source = `${metadataSource} ${asset.source || ""}`.trim().toLowerCase();
    const generatedMarkers = ["generation", "generated", "task-result", "tasks-page", "ai result", "ai 生成", "任务生成"];
    return generatedMarkers.some((marker) => source.includes(marker)) ? "works" : "materials";
}
