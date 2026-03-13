import * as React from "react"
import * as CheckboxPrimitive from "@radix-ui/react-checkbox"
import { Check } from "lucide-react"
import { cn } from "@/lib/utils"

const Checkbox = React.forwardRef<
  React.ComponentRef<typeof CheckboxPrimitive.Root>,
  React.ComponentPropsWithoutRef<typeof CheckboxPrimitive.Root> & { label?: string }
>(({ className, label, ...props }, ref) => {
  const id = React.useId()
  const checkbox = (
    <CheckboxPrimitive.Root
      ref={ref}
      id={label ? id : undefined}
      className={cn(
        "peer h-4 w-4 shrink-0 rounded-sm border border-primary shadow focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring disabled:cursor-not-allowed disabled:opacity-50 data-[state=checked]:bg-primary data-[state=checked]:text-primary-foreground cursor-pointer",
        className
      )}
      {...props}
    >
      <CheckboxPrimitive.Indicator className={cn("flex items-center justify-center text-current")}>
        <Check className="h-3.5 w-3.5" />
      </CheckboxPrimitive.Indicator>
    </CheckboxPrimitive.Root>
  )

  if (label) {
    return (
      <div className="flex items-center gap-2">
        {checkbox}
        <label htmlFor={id} className="text-sm leading-none cursor-pointer select-none">{label}</label>
      </div>
    )
  }

  return checkbox
})
Checkbox.displayName = CheckboxPrimitive.Root.displayName

export { Checkbox }
