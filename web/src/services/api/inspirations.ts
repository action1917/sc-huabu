import { apiClient, compactApiParams, request } from "@/services/api/request";

export type InspirationKind = "image" | "video" | "short-drama";

export type InspirationItem = {
    id: string;
    kind: InspirationKind;
    title: string;
    description: string;
    previewUrl: string;
    promptPreview?: string;
    parametersPreview?: Record<string, unknown>;
    sourceType: string;
    priceMicrocredits: number;
    copyCount: number;
    owner: boolean;
    unlocked: boolean;
    favorite: boolean;
    favoriteCount: number;
    authorName: string;
    createdAt: string;
};

export type InspirationReplica = {
    id: string;
    kind: InspirationKind;
    prompt: string;
    parameters: Record<string, unknown>;
    sourceType: string;
    sourceId: string;
    purchased: boolean;
};

export function listInspirations(input: { scope?: "public" | "favorites"; kind?: string; keyword?: string; limit?: number } = {}) {
    return request<{ items: InspirationItem[] }>(apiClient.get("/inspirations", { params: compactApiParams(input) }));
}

export function createInspiration(input: {
    kind: InspirationKind;
    title: string;
    description?: string;
    previewUrl?: string;
    previewResourceId?: string;
    prompt: string;
    parameters: Record<string, unknown>;
    sourceType: string;
    sourceId?: string;
    priceMicrocredits: number;
}) {
    return request<{ item: InspirationItem }>(apiClient.post("/inspirations", input));
}

export function replicateInspiration(id: string) {
    return request<{ replica: InspirationReplica }>(apiClient.post(`/inspirations/${encodeURIComponent(id)}/replicate`));
}

export function unlockInspiration(id: string) {
    return request<{ unlocked: boolean; purchased: boolean }>(apiClient.post(`/inspirations/${encodeURIComponent(id)}/unlock`));
}

export function setInspirationFavorite(id: string, favorite: boolean) {
    return request<{ favorite: boolean }>(apiClient.patch(`/inspirations/${encodeURIComponent(id)}/favorite`, { favorite }));
}
