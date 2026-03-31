import { useState, useRef, useEffect } from "react";
import { Plus, ArrowUp } from "lucide-react";

interface ChatInputProps {
  onSend: (message: string) => void;
  disabled?: boolean;
  centered?: boolean;
}

export default function ChatInput({ onSend, disabled, centered }: ChatInputProps) {
  const [value, setValue] = useState("");
  const textareaRef = useRef<HTMLTextAreaElement>(null);

  useEffect(() => {
    if (textareaRef.current) {
      textareaRef.current.style.height = "auto";
      textareaRef.current.style.height = Math.min(textareaRef.current.scrollHeight, 200) + "px";
    }
  }, [value]);

  const handleSubmit = () => {
    const trimmed = value.trim();
    if (!trimmed || disabled) return;
    onSend(trimmed);
    setValue("");
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      handleSubmit();
    }
  };

  return (
    <div className={`mx-auto w-full max-w-[680px] px-3 ${centered ? "" : "pb-4 pt-2"}`}>
      <div className="relative rounded-3xl border border-[hsl(var(--chat-input-border))] bg-[hsl(var(--chat-input-bg))] shadow-[0_1px_6px_rgba(0,0,0,0.04)] transition-shadow focus-within:shadow-[0_1px_10px_rgba(0,0,0,0.08)]">
        <div className="flex items-end px-3 py-2">
          {/* Plus button */}
          <button className="mb-1 flex h-8 w-8 shrink-0 items-center justify-center rounded-full text-foreground/60 hover:text-foreground hover:bg-[hsl(var(--accent))] transition-colors">
            <Plus className="h-5 w-5" strokeWidth={1.8} />
          </button>

          {/* Textarea */}
          <textarea
            ref={textareaRef}
            value={value}
            onChange={(e) => setValue(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder="Vibe Coding 快速实现构想"
            rows={1}
            disabled={disabled}
            className="flex-1 resize-none bg-transparent px-2 py-1.5 text-[15px] text-foreground placeholder:text-[hsl(var(--sidebar-muted))] focus:outline-none max-h-[200px] leading-6"
          />

          {/* Right buttons */}
          <div className="mb-1 flex items-center gap-1">
            <button
              onClick={handleSubmit}
              disabled={disabled || !value.trim()}
              className="flex h-8 w-8 items-center justify-center rounded-full bg-foreground text-background hover:bg-foreground/80 disabled:opacity-30 transition-colors"
            >
              <ArrowUp className="h-4 w-4" strokeWidth={2.5} />
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}
