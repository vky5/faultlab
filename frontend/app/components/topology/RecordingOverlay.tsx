import { AnimatePresence, motion } from "framer-motion";

interface RecordingOverlayProps {
  isRecording: boolean;
  frameCount: number;
}

export function RecordingOverlay({ isRecording, frameCount }: RecordingOverlayProps) {
  return (
    <AnimatePresence>
      {isRecording && (
        <motion.div
          initial={{ opacity: 0, x: -20 }}
          animate={{ opacity: 1, x: 0 }}
          exit={{ opacity: 0, x: -20 }}
          className="absolute top-4 left-4 z-50 flex items-center gap-3 bg-red-600 text-white px-4 py-2 rounded-full shadow-lg border border-red-500/50 backdrop-blur-md recording-overlay"
        >
          <div className="w-2 h-2 rounded-full bg-white animate-pulse" />
          <div className="flex flex-col">
            <span className="text-[10px] font-black uppercase tracking-widest leading-none">
              Recording
            </span>
            <span className="text-[9px] font-mono opacity-80">
              {frameCount} frames captured
            </span>
          </div>
        </motion.div>
      )}
    </AnimatePresence>
  );
}
