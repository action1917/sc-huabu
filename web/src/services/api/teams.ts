import { apiClient, compactApiParams, request } from "@/services/api/request";

export type Team = { id: string; ownerUserId: string; name: string; description: string; joinCode: string; role?: "owner" | "member"; memberCount?: number; createdAt: string; updatedAt: string };
export type TeamMember = { id: string; username: string; displayName: string; avatarUrl?: string; role: "owner" | "member" };
export type TeamResource = { id: string; teamId: string; sourceType: "resource" | "asset" | "work" | "project" | "inspiration" | "skill"; sourceId: string; sharedBy: string; sharerName: string; title: string; previewUrl?: string; createdAt: string };

export function listTeams() {
    return request<{ items: Team[] }>(apiClient.get("/teams"));
}
export function createTeam(input: { name: string; description?: string }) {
    return request<{ team: Team }>(apiClient.post("/teams", input));
}
export function joinTeam(code: string) {
    return request<{ team: Team; joined: boolean }>(apiClient.post("/teams/join", { code }));
}
export function getTeam(id: string) {
    return request<{ team: Team; members: TeamMember[] }>(apiClient.get(`/teams/${encodeURIComponent(id)}`));
}
export function leaveTeam(id: string) {
    return request<{ left: boolean }>(apiClient.post(`/teams/${encodeURIComponent(id)}/leave`));
}
export function listTeamResources(id: string, type = "all") {
    return request<{ items: TeamResource[] }>(apiClient.get(`/teams/${encodeURIComponent(id)}/resources`, { params: compactApiParams({ type }) }));
}
export function shareTeamResource(id: string, input: { sourceType: TeamResource["sourceType"]; sourceId: string; title?: string; previewUrl?: string; metadata?: Record<string, unknown> }) {
    return request<{ item: TeamResource }>(apiClient.post(`/teams/${encodeURIComponent(id)}/resources`, input));
}
export function unshareTeamResource(id: string, type: string, sourceId: string) {
    return request<{ deleted: boolean }>(apiClient.delete(`/teams/${encodeURIComponent(id)}/resources/${encodeURIComponent(type)}/${encodeURIComponent(sourceId)}`));
}
export function getTeamResourceContent(id: string, type: string, sourceId: string) {
    return request<{ content: Record<string, unknown> }>(apiClient.get(`/teams/${encodeURIComponent(id)}/resources/${encodeURIComponent(type)}/${encodeURIComponent(sourceId)}/content`));
}
