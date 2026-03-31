import { PenSquare, Search, PanelLeftClose, PanelLeft, Settings, HelpCircle, Trash2 } from "lucide-react";
import { cn } from "@/lib/utils";

export interface Conversation {
  id: string;
  title: string;
}

interface ChatSidebarProps {
  conversations: Conversation[];
  activeId: string | null;
  onSelect: (id: string) => void;
  onNewChat: () => void;
  onDelete: (id: string) => void;
  isOpen: boolean;
  onToggle: () => void;
}

const navItems = [
  { icon: PenSquare, label: "新建对话", action: "new" },
  { icon: Search, label: "搜索对话", action: "search" }
];

const bottomItems = [
  { icon: Settings, label: "设置" },
  { icon: HelpCircle, label: "帮助" },
];

export default function ChatSidebar({
  conversations,
  activeId,
  onSelect,
  onNewChat,
  onDelete,
  isOpen,
  onToggle,
}: ChatSidebarProps) {
  if (!isOpen) {
    return (
      <div className="absolute left-2 top-2 z-20">
        <button onClick={onToggle} className="flex h-10 w-10 items-center justify-center rounded-lg text-foreground/70 hover:bg-[hsl(var(--sidebar-hover))] transition-colors">
          <PanelLeft className="h-[18px] w-[18px]" />
        </button>
      </div>
    );
  }

  return (
    <aside className="flex h-full w-[200px] flex-col bg-[hsl(var(--sidebar-bg))] border-r border-[hsl(var(--border))]">
      {/* Top section */}
      <div className="flex items-center justify-between px-2 pt-2">
        <button onClick={onToggle} className="flex h-10 w-10 items-center justify-center rounded-lg text-foreground/70 hover:bg-[hsl(var(--sidebar-hover))] transition-colors">
          <PanelLeftClose className="h-[18px] w-[18px]" />
        </button>
      </div>

      {/* Nav items */}
      <nav className="px-2 pt-1 space-y-0.5">
        {navItems.map((item) => (
          <button
            key={item.label}
            onClick={item.action === "new" ? onNewChat : undefined}
            className="flex w-full items-center gap-3 rounded-lg px-3 py-2 text-sm text-foreground hover:bg-[hsl(var(--sidebar-hover))] transition-colors"
          >
            <item.icon className="h-[18px] w-[18px]" strokeWidth={1.8} />
            <span>{item.label}</span>
          </button>
        ))}
      </nav>

      {/* Conversations */}
      {conversations.length > 0 ? (
        <div className="mt-4 flex-1 overflow-y-auto px-2">
          <p className="px-3 pb-1.5 text-xs font-medium text-[hsl(var(--sidebar-muted))]">最近</p>
          <div className="space-y-0.5">
            {conversations.map((conv) => (
              <div
                key={conv.id}
                className={cn(
                  "group flex w-full items-center gap-2 rounded-lg px-3 py-2 text-sm transition-colors",
                  activeId === conv.id
                    ? "bg-[hsl(var(--sidebar-active))] text-foreground"
                    : "text-foreground hover:bg-[hsl(var(--sidebar-hover))]"
                )}
              >
                <button
                  onClick={() => onSelect(conv.id)}
                  className="flex-1 text-left truncate"
                >
                  <span className="truncate">{conv.title}</span>
                </button>
                <button
                  onClick={(e) => {
                    e.stopPropagation();
                    onDelete(conv.id);
                  }}
                  className="opacity-0 group-hover:opacity-100 flex h-6 w-6 items-center justify-center rounded hover:bg-[hsl(var(--sidebar-hover))] transition-opacity"
                  aria-label="删除对话"
                >
                  <Trash2 className="h-4 w-4" strokeWidth={1.8} />
                </button>
              </div>
            ))}
          </div>
        </div>
      ) : (
        <div className="mt-4 flex-1 px-2">
          <p className="px-3 py-4 text-xs text-[hsl(var(--sidebar-muted))] text-center">
            暂无对话<br />开始新对话吧
          </p>
        </div>
      )}

      {/* Bottom section */}
      <div className="mt-auto px-2 pb-2 space-y-0.5">
        {bottomItems.map((item) => (
          <button
            key={item.label}
            className="flex w-full items-center gap-3 rounded-lg px-3 py-2 text-sm text-foreground hover:bg-[hsl(var(--sidebar-hover))] transition-colors"
          >
            <item.icon className="h-[18px] w-[18px]" strokeWidth={1.8} />
            <span>{item.label}</span>
          </button>
        ))}

        
      </div>
    </aside>
  );
}
