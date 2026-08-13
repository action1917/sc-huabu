import { motion, useReducedMotion } from "motion/react";

import { cn } from "@/lib/utils";

type BrandLogoProps = {
    className?: string;
    animated?: boolean;
    label?: string;
};

export function BrandLogo({ className, animated = true, label }: BrandLogoProps) {
    const reducedMotion = useReducedMotion();
    const motionEnabled = animated && !reducedMotion;

    return (
        <motion.span
            className={cn("app-brand-logo relative inline-grid shrink-0 place-items-center", motionEnabled && "is-animated", className)}
            role={label ? "img" : undefined}
            aria-label={label}
            aria-hidden={label ? undefined : true}
            animate={motionEnabled ? { y: [0, -1.5, 0], rotate: [0, -0.8, 0, 0.8, 0] } : undefined}
            transition={motionEnabled ? { duration: 4.8, repeat: Infinity, ease: "easeInOut" } : undefined}
            whileHover={reducedMotion ? undefined : { scale: 1.08, rotate: 2 }}
        >
            <img src="/suchuang-logo.png" alt="" draggable={false} className="relative z-[1] size-full select-none object-contain" />
        </motion.span>
    );
}
