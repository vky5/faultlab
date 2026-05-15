declare module "gifshot" {
  export function createGIF(options: {
    images: string[];
    gifWidth: number;
    gifHeight: number;
    interval: number;
    numWorkers: number;
  }, callback: (result: { error: boolean; image: string }) => void): void;
}
