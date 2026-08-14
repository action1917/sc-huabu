import { create } from "zustand";

import { DEFAULT_DRAWING_ENGINE, type CanvasDrawingEngineSetting } from "@/lib/canvas/canvas-drawing-engine";

export type LocalUser = {
    id: string;
    username: string;
    email?: string;
    displayName: string;
    avatarUrl?: string;
    identityProvider?: string;
    identityId?: string;
    identityUsername?: string;
    role: "admin" | "user";
    status: "active" | "disabled";
    lastLoginAt?: string;
    createdAt?: string;
    updatedAt?: string;
};

export type RuntimeLimits = {
    activeTaskLimit: number;
    resourceUploadMB: number;
    sessionUploadMB: number;
};

export type FeatureAvailability = {
    creationEnabled: boolean;
    shortDramaEnabled: boolean;
    inspirationsEnabled: boolean;
    canvasEnabled: boolean;
    taskCenterEnabled: boolean;
    assetsEnabled: boolean;
    skillsEnabled: boolean;
    teamsEnabled: boolean;
    creditsEnabled: boolean;
    settingsEnabled: boolean;
    configured?: boolean;
    updatedBy?: string;
    updatedAt?: string;
};

export type FeatureKey = Exclude<keyof FeatureAvailability, "configured" | "updatedBy" | "updatedAt">;

export const defaultFeatureAvailability: FeatureAvailability = {
    creationEnabled: true,
    shortDramaEnabled: true,
    inspirationsEnabled: true,
    canvasEnabled: true,
    taskCenterEnabled: true,
    assetsEnabled: true,
    skillsEnabled: true,
    teamsEnabled: true,
    creditsEnabled: true,
    settingsEnabled: true,
};

type UserStore = {
    hydrated: boolean;
    user: LocalUser | null;
    runtimeLimits: RuntimeLimits;
    drawingEngine: CanvasDrawingEngineSetting;
    features: FeatureAvailability;
    setUser: (user: LocalUser | null) => void;
    setRuntimeLimits: (limits?: RuntimeLimits) => void;
    setDrawingEngine: (setting?: CanvasDrawingEngineSetting) => void;
    setFeatures: (features?: FeatureAvailability) => void;
    setHydrated: (hydrated: boolean) => void;
    clearSession: () => void;
};

export const useUserStore = create<UserStore>()((set) => ({
    hydrated: false,
    user: null,
    runtimeLimits: { activeTaskLimit: 5, resourceUploadMB: 50, sessionUploadMB: 32 },
    drawingEngine: { defaultEngine: DEFAULT_DRAWING_ENGINE },
    features: defaultFeatureAvailability,
    setUser: (user) => set({ user }),
    setRuntimeLimits: (runtimeLimits) => set({ runtimeLimits: runtimeLimits || { activeTaskLimit: 5, resourceUploadMB: 50, sessionUploadMB: 32 } }),
    setDrawingEngine: (drawingEngine) => set({ drawingEngine: drawingEngine || { defaultEngine: DEFAULT_DRAWING_ENGINE } }),
    setFeatures: (features) => set({ features: features || defaultFeatureAvailability }),
    setHydrated: (hydrated) => set({ hydrated }),
    clearSession: () => set({ user: null, runtimeLimits: { activeTaskLimit: 5, resourceUploadMB: 50, sessionUploadMB: 32 }, drawingEngine: { defaultEngine: DEFAULT_DRAWING_ENGINE }, features: defaultFeatureAvailability }),
}));
