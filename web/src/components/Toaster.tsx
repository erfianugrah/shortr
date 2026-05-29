import { Toaster as SonnerToaster } from "@/components/ui/sonner";

// Single mount point for the toast layer. Keeps the layout JSX-free.
export default function Toaster() {
  return <SonnerToaster position="top-right" richColors closeButton />;
}
