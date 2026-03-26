import { useEffect, useRef } from 'react';
import { useClusterStore } from '../store';

export function useRaftSimulation() {
  const tickSimulation = useClusterStore((state) => state.tickSimulation);
  const isSimulationRunning = useClusterStore((state) => state.isSimulationRunning);
  const lastTimeRef = useRef<number>(0);
  const frameRef = useRef<number>(0);

  useEffect(() => {
    if (!isSimulationRunning) {
      if (frameRef.current) cancelAnimationFrame(frameRef.current);
      return;
    }

    const loop = (time: number) => {
      if (lastTimeRef.current !== 0) {
        const deltaMs = time - lastTimeRef.current;
        // Cap max delta to prevent huge jumps when tabbing away
        tickSimulation(Math.min(deltaMs, 100));
      }
      lastTimeRef.current = time;
      frameRef.current = requestAnimationFrame(loop);
    };

    lastTimeRef.current = 0; // reset on start
    frameRef.current = requestAnimationFrame(loop);

    return () => {
      cancelAnimationFrame(frameRef.current);
    };
  }, [isSimulationRunning, tickSimulation]);
}
