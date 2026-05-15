interface LegendPanelProps {
  messageCount: number;
  isPaused: boolean;
}

export function LegendPanel({ messageCount, isPaused }: LegendPanelProps) {
  const legendItems = [
    { color: "#eab308", shadow: "#eab308", label: "Gossip Digest" },
    { color: "#10b981", shadow: "#10b981", label: "Gossip State" },
    { color: "#3b82f6", shadow: "#3b82f6", label: "Sync Request" },
    { color: "#a855f7", shadow: "#a855f7", label: "Node Reg" },
    { color: "#0ea5e9", shadow: "#0ea5e9", label: "KV Put (CP)" },
    { color: "#f97316", shadow: "#f97316", label: "KV Get (CP)" },
  ];

  return (
    <div className="absolute bottom-4 right-4 bg-white/95 dark:bg-slate-800/95 backdrop-blur-sm border border-slate-200 dark:border-slate-700 rounded-xl p-4 shadow-xl pointer-events-auto z-50">
      <div className="text-[10px] font-bold text-slate-500 uppercase tracking-wider mb-3">
        Protocol Messages
      </div>
      <div className="grid grid-cols-2 gap-x-6 gap-y-2">
        {legendItems.map(({ color, shadow, label }) => (
          <div key={label} className="flex items-center gap-2">
            <div
              className="w-2.5 h-2.5 rounded-full"
              style={{
                backgroundColor: color,
                filter: `drop-shadow(0 0 6px ${shadow})`,
              }}
            />
            <span className="text-[11px] text-slate-600 dark:text-slate-400">{label}</span>
          </div>
        ))}
      </div>
      <div className="mt-3 pt-3 border-t border-slate-200 dark:border-slate-700 flex items-center justify-between">
        <div className="text-[10px] text-slate-500 font-medium">
          In-flight: <span className="font-mono font-bold text-primary">{messageCount}</span>
        </div>
        <div
          className={`text-[10px] px-2 py-0.5 rounded-full font-bold uppercase tracking-tighter ${
            isPaused ? "bg-warning/20 text-warning" : "bg-success/20 text-success"
          }`}
        >
          {isPaused ? "Paused" : "Live"}
        </div>
      </div>
    </div>
  );
}
