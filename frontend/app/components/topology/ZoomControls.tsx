import { BarChart3, Crown, FileImage, Activity } from "lucide-react";
import { motion } from "framer-motion";

interface ZoomControlsProps {
  handleZoomIn: () => void;
  handleZoomOut: () => void;
  handleResetZoom: () => void;
  toggleControlPlane: () => void;
  showControlPlane: boolean;
  showMetrics: boolean;
  setShowMetrics: (value: boolean) => void;
  isRecording: boolean;
  stopRecording: () => void;
  startRecording: () => void;
}

export function ZoomControls({
  handleZoomIn,
  handleZoomOut,
  handleResetZoom,
  toggleControlPlane,
  showControlPlane,
  showMetrics,
  setShowMetrics,
  isRecording,
  stopRecording,
  startRecording,
}: ZoomControlsProps) {
  return (
    <div className="absolute bottom-4 left-4 flex flex-col gap-2 z-50">
      <div className="flex flex-col bg-white/95 dark:bg-slate-800/95 backdrop-blur-sm border border-slate-200 dark:border-slate-700 rounded-xl overflow-hidden shadow-xl">
        <button
          onClick={handleZoomIn}
          className="p-2.5 hover:bg-slate-100 dark:hover:bg-slate-700 transition-colors border-b border-slate-200 dark:border-slate-700"
          title="Zoom In"
        >
          <div className="w-5 h-5 flex items-center justify-center font-bold text-slate-600 dark:text-slate-300">＋</div>
        </button>
        <button
          onClick={handleZoomOut}
          className="p-2.5 hover:bg-slate-100 dark:hover:bg-slate-700 transition-colors border-b border-slate-200 dark:border-slate-700"
          title="Zoom Out"
        >
          <div className="w-5 h-5 flex items-center justify-center font-bold text-slate-600 dark:text-slate-300">－</div>
        </button>
        <button
          onClick={handleResetZoom}
          className="p-2.5 hover:bg-slate-100 dark:hover:bg-slate-700 transition-colors border-b border-slate-200 dark:border-slate-700"
          title="Reset View"
        >
          <Activity className="w-4 h-4 text-slate-600 dark:text-slate-300" />
        </button>
        <button
          onClick={toggleControlPlane}
          className={`p-2.5 hover:bg-slate-100 dark:hover:bg-slate-700 transition-colors ${
            showControlPlane ? "text-primary" : "text-slate-400"
          }`}
          title={showControlPlane ? "Hide Hub" : "Show Hub"}
        >
          <Crown className="w-4 h-4" />
        </button>

        <motion.button
          whileHover={{ scale: 1.1 }}
          whileTap={{ scale: 0.9 }}
          onClick={() => setShowMetrics(!showMetrics)}
          title="Metrics Analysis"
          className={`p-2.5 rounded-xl border transition-all ${
            showMetrics
              ? "bg-primary text-white border-primary shadow-lg shadow-primary/30"
              : "bg-white/90 dark:bg-slate-900/90 backdrop-blur-xl border-slate-200 dark:border-slate-800 text-slate-700 dark:text-slate-200 hover:border-primary/50"
          }`}
        >
          <BarChart3 className={`w-4 h-4 ${showMetrics ? "animate-pulse" : ""}`} />
        </motion.button>

        <motion.button
          whileHover={{ scale: 1.1 }}
          whileTap={{ scale: 0.9 }}
          onClick={isRecording ? stopRecording : startRecording}
          title={isRecording ? "Stop Recording" : "Record GIF"}
          className={`p-2.5 rounded-xl border transition-all ${
            isRecording
              ? "bg-red-600 text-white border-red-500 shadow-lg shadow-red-500/30 animate-pulse"
              : "bg-white/90 dark:bg-slate-900/90 backdrop-blur-xl border-slate-200 dark:border-slate-800 text-slate-700 dark:text-slate-200 hover:border-red-500/50"
          }`}
        >
          <FileImage className={`w-4 h-4 ${isRecording ? "animate-spin-slow" : ""}`} />
        </motion.button>
      </div>
    </div>
  );
}
