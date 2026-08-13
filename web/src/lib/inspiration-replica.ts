import type { InspirationReplica } from "@/services/api/inspirations";

export const INSPIRATION_REPLICA_STORAGE_KEY = "infinite-canvas:inspiration-replica";

export function saveInspirationReplica(replica: InspirationReplica) {
    window.sessionStorage.setItem(INSPIRATION_REPLICA_STORAGE_KEY, JSON.stringify(replica));
}

export function takeInspirationReplica(): InspirationReplica | null {
    const raw = window.sessionStorage.getItem(INSPIRATION_REPLICA_STORAGE_KEY);
    if (!raw) return null;
    window.sessionStorage.removeItem(INSPIRATION_REPLICA_STORAGE_KEY);
    try {
        return JSON.parse(raw) as InspirationReplica;
    } catch {
        return null;
    }
}
