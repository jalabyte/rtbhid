import {
  useState,
  type ComponentProps,
  type ComponentType,
  type ReactNode,
} from "react";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { TableCell, TableRow } from "@/components/ui/table";
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { cn } from "@/lib/utils";

export function PageHeader({
  title,
  description,
  children,
}: {
  title: ReactNode;
  description?: ReactNode;
  children?: ReactNode;
}) {
  return (
    <header className="flex flex-wrap items-center gap-x-3 gap-y-2">
      <div className="mr-auto min-w-0">
        <h1 className="text-lg font-semibold leading-tight">{title}</h1>
        {description && (
          <p className="text-muted-foreground">{description}</p>
        )}
      </div>
      {children}
    </header>
  );
}

// Field gives every input a visible label above it.
export function Field({
  label,
  htmlFor,
  className,
  children,
}: {
  label: string;
  htmlFor: string;
  className?: string;
  children: ReactNode;
}) {
  return (
    <div className={cn("grid gap-1", className)}>
      <Label htmlFor={htmlFor} className="text-xs text-muted-foreground">
        {label}
      </Label>
      {children}
    </div>
  );
}

// FormRow is an add form: Enter submits, fields wrap on narrow screens.
export function FormRow({
  onSubmit,
  children,
  className,
}: {
  onSubmit: () => void;
  children: ReactNode;
  className?: string;
}) {
  return (
    <form
      className={cn("flex flex-wrap items-end gap-2", className)}
      onSubmit={(e) => {
        e.preventDefault();
        onSubmit();
      }}
    >
      {children}
    </form>
  );
}

export function ErrorNote({ children }: { children: ReactNode }) {
  if (!children) return null;
  return (
    <p role="alert" className="text-destructive">
      {children}
    </p>
  );
}

// TableState renders the loading / error / empty row of a table.
export function TableState({
  colSpan,
  loading,
  error,
  empty,
  emptyText,
  onRetry,
}: {
  colSpan: number;
  loading: boolean;
  error: string;
  empty: boolean;
  emptyText: ReactNode;
  onRetry?: () => void;
}) {
  let content: ReactNode = null;
  if (error && empty) {
    content = (
      <span role="alert" className="text-destructive">
        Could not load: {error}
        {onRetry && (
          <Button variant="link" size="sm" className="h-auto" onClick={onRetry}>
            Retry
          </Button>
        )}
      </span>
    );
  } else if (loading && empty) {
    content = <span className="text-muted-foreground">Loading…</span>;
  } else if (empty) {
    content = <span className="text-muted-foreground">{emptyText}</span>;
  }
  if (!content) return null;
  return (
    <TableRow className="hover:bg-transparent">
      <TableCell colSpan={colSpan} className="py-6 text-center whitespace-normal">
        {content}
      </TableCell>
    </TableRow>
  );
}

// StatusBadge marks on/off with shape and text, not colour alone: a filled
// dot for on, a hollow ring for off.
export function StatusBadge({
  on,
  onLabel = "enabled",
  offLabel = "disabled",
}: {
  on: boolean;
  onLabel?: string;
  offLabel?: string;
}) {
  return (
    <span
      className={cn(
        "inline-flex items-center gap-1.5 text-xs font-medium",
        on ? "text-signal" : "text-muted-foreground",
      )}
    >
      <span
        aria-hidden="true"
        className={cn(
          "size-2 rounded-full",
          on ? "bg-signal" : "border border-current",
        )}
      />
      {on ? onLabel : offLabel}
    </span>
  );
}

export function IconButton({
  label,
  icon: Icon,
  className,
  ...props
}: {
  label: string;
  icon: ComponentType<{ className?: string }>;
} & ComponentProps<typeof Button>) {
  return (
    <Button
      variant="ghost"
      size="icon-sm"
      aria-label={label}
      title={label}
      className={className}
      {...props}
    >
      <Icon className="size-4" aria-hidden="true" />
    </Button>
  );
}

// Confirm wraps a trigger and asks before running a destructive action.
export function Confirm({
  title,
  description,
  confirmLabel,
  onConfirm,
  children,
}: {
  title: string;
  description: ReactNode;
  confirmLabel: string;
  onConfirm: () => unknown;
  children: ReactNode;
}) {
  const [open, setOpen] = useState(false);
  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild>{children}</DialogTrigger>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle className="text-base">{title}</DialogTitle>
          <DialogDescription>{description}</DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <DialogClose asChild>
            <Button variant="outline">Cancel</Button>
          </DialogClose>
          <Button
            variant="destructive"
            onClick={() => {
              setOpen(false);
              onConfirm();
            }}
          >
            {confirmLabel}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

// Brand mark (brand/nullwell-symbol.svg): the n in the theme's text colour,
// the null point in the signal accent.
export function NullwellMark({ className }: { className?: string }) {
  return (
    <svg viewBox="0 0 256 256" className={className} aria-hidden="true">
      <path fill="currentColor" d="M40 224V116A88 88 0 0 1 216 116V224H176V112A48 48 0 0 0 80 112V224Z" />
      <circle cx="128" cy="202" r="24" className="fill-signal" />
    </svg>
  );
}

