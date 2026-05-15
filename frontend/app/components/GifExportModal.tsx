import React from "react";
import { Download, X, Loader2, FileImage, CheckCircle2, AlertCircle } from "lucide-react";
import { motion, AnimatePresence } from "framer-motion";
import { useClusterStore } from "../store";

export function GifExportModal() {
  const { 
    gifUrl, 
    isEncoding, 
    recordedFrames, 
    clearRecording, 
    setGifUrl 
  } = useClusterStore();

  const isOpen = isEncoding || !!gifUrl;

  if (!isOpen) return null;

  const downloadGif = () => {
    if (!gifUrl) return;
    const link = document.createElement("a");
    link.href = gifUrl;
    link.download = `faultlab-simulation-${new Date().toISOString().split('.')[0]}.gif`;
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
  };

  return (
    <div className="fixed inset-0 z-[100] flex items-center justify-center p-4">
      <motion.div 
        initial={{ opacity: 0 }}
        animate={{ opacity: 1 }}
        exit={{ opacity: 0 }}
        onClick={clearRecording}
        className="absolute inset-0 bg-slate-900/60 backdrop-blur-sm"
      />
      
      <motion.div 
        initial={{ opacity: 0, scale: 0.9, y: 20 }}
        animate={{ opacity: 1, scale: 1, y: 0 }}
        exit={{ opacity: 0, scale: 0.9, y: 20 }}
        className="relative w-full max-w-lg bg-white dark:bg-slate-900 rounded-3xl shadow-2xl border border-slate-200 dark:border-slate-800 overflow-hidden"
      >
        {/* Header */}
        <div className="px-6 py-4 border-b border-slate-100 dark:border-slate-800 flex items-center justify-between bg-slate-50/50 dark:bg-slate-800/50">
          <div className="flex items-center gap-2">
            <FileImage className="w-5 h-5 text-primary" />
            <h3 className="font-bold text-slate-800 dark:text-slate-100">GIF Export</h3>
          </div>
          <button 
            onClick={clearRecording}
            className="p-1.5 hover:bg-slate-200 dark:hover:bg-slate-800 rounded-full transition-colors text-slate-400 hover:text-slate-600 dark:hover:text-slate-200"
          >
            <X className="w-5 h-5" />
          </button>
        </div>

        <div className="p-8">
          {isEncoding ? (
            <div className="flex flex-col items-center text-center space-y-6">
              <div className="relative">
                <div className="w-20 h-20 rounded-full border-4 border-primary/10 flex items-center justify-center">
                  <Loader2 className="w-10 h-10 text-primary animate-spin" />
                </div>
                <div className="absolute -bottom-1 -right-1 bg-white dark:bg-slate-900 p-1 rounded-full border border-slate-200 dark:border-slate-800 shadow-sm">
                  <div className="w-6 h-6 bg-primary rounded-full flex items-center justify-center text-[10px] font-bold text-white">
                    {Math.min(99, Math.floor((recordedFrames.length / 50) * 100))}%
                  </div>
                </div>
              </div>
              <div>
                <h4 className="text-xl font-bold text-slate-800 dark:text-slate-100 mb-2">Encoding your simulation...</h4>
                <p className="text-slate-500 dark:text-slate-400 text-sm max-w-xs mx-auto">
                  Processing {recordedFrames.length} frames into a high-quality GIF. This may take a moment depending on the complexity.
                </p>
              </div>
              <div className="w-full h-2 bg-slate-100 dark:bg-slate-800 rounded-full overflow-hidden">
                <motion.div 
                  className="h-full bg-primary"
                  initial={{ width: 0 }}
                  animate={{ width: "100%" }}
                  transition={{ duration: 10, ease: "linear" }}
                />
              </div>
            </div>
          ) : gifUrl ? (
            <div className="flex flex-col items-center text-center space-y-6">
              <div className="relative group">
                <div className="w-full rounded-2xl overflow-hidden border border-slate-200 dark:border-slate-800 shadow-lg bg-slate-50 dark:bg-slate-800 max-h-64 flex items-center justify-center">
                  <img src={gifUrl} alt="Exported Simulation" className="max-w-full max-h-full object-contain" />
                </div>
                <div className="absolute inset-0 bg-primary/20 opacity-0 group-hover:opacity-100 transition-opacity flex items-center justify-center">
                  <CheckCircle2 className="w-12 h-12 text-white drop-shadow-lg" />
                </div>
              </div>
              
              <div>
                <h4 className="text-xl font-bold text-slate-800 dark:text-slate-100 mb-2">Export Ready!</h4>
                <p className="text-slate-500 dark:text-slate-400 text-sm">
                  Your simulation has been successfully captured and encoded.
                </p>
              </div>

              <div className="flex gap-3 w-full">
                <button 
                  onClick={clearRecording}
                  className="flex-1 py-3 px-4 rounded-xl border border-slate-200 dark:border-slate-800 font-bold text-slate-600 dark:text-slate-400 hover:bg-slate-50 dark:hover:bg-slate-800 transition-colors"
                >
                  Discard
                </button>
                <button 
                  onClick={downloadGif}
                  className="flex-2 py-3 px-6 rounded-xl bg-primary text-white font-bold hover:bg-primary/90 transition-all shadow-lg shadow-primary/30 flex items-center justify-center gap-2"
                >
                  <Download className="w-5 h-5" />
                  Download GIF
                </button>
              </div>
            </div>
          ) : (
            <div className="flex flex-col items-center text-center py-8 space-y-4">
              <AlertCircle className="w-12 h-12 text-destructive opacity-50" />
              <p className="text-slate-500">Something went wrong with the export.</p>
              <button onClick={clearRecording} className="btn-secondary">Close</button>
            </div>
          )}
        </div>

        <div className="px-6 py-3 bg-slate-50 dark:bg-slate-800/50 border-t border-slate-100 dark:border-slate-800 flex items-center justify-center gap-2 text-[10px] text-slate-400 uppercase font-bold tracking-widest">
          Faultlab Simulation Capture v1.0
        </div>
      </motion.div>
    </div>
  );
}
