import { useState, useEffect } from "react";
import { useNavigate } from "react-router-dom";
import { ArrowRight, Sparkles, Zap, Code, MessageSquare } from "lucide-react";
import ChatInput from "@/components/ChatInput";
import { apiClient } from "@/lib/api";
import { toast } from "sonner";

interface Project {
  id: string;
  name: string;
  icon: string;
  editedTime: string;
  thumbnail: string;
  badge?: string;
  description?: string;
}

export default function Home() {
  const navigate = useNavigate();
  const [projects, setProjects] = useState<Project[]>([]);
  const [activeTab, setActiveTab] = useState<"projects" | "recent" | "templates">("projects");
  const [isLoading, setIsLoading] = useState(false);

  useEffect(() => {
    loadProjects();
  }, []);

  const loadProjects = async () => {
    try {
      const response = await apiClient.listSessions(50, 0);
      const loadedProjects: Project[] = (response.items || []).map((item: any) => ({
        id: item.conversation_id || item.project_id,
        name: item.first_message || item.name || "未命名项目",
        icon: item.icon || "💬",
        editedTime: formatTime(item.updated_at || item.last_message_timestamp),
        thumbnail: item.thumbnail || "bg-gradient-to-br from-gray-100 to-gray-200",
        description: item.description,
      }));
      setProjects(loadedProjects);
    } catch (error) {
      console.error("Failed to load projects:", error);
    }
  };

  const formatTime = (timestamp: string) => {
    if (!timestamp) return "刚刚";
    const date = new Date(timestamp);
    const now = new Date();
    const diff = now.getTime() - date.getTime();
    const hours = Math.floor(diff / (1000 * 60 * 60));
    const days = Math.floor(diff / (1000 * 60 * 60 * 24));
    
    if (hours < 1) return "刚刚";
    if (hours < 24) return `${hours} 小时前`;
    if (days < 30) return `${days} 天前`;
    return date.toLocaleDateString();
  };

  const handleSend = async (text: string) => {
    setIsLoading(true);
    try {
      const response = await apiClient.createSession();
      const projectId = response.conversation_id;
      navigate(`/project/${projectId}`);
      
      setTimeout(async () => {
        try {
          await apiClient.sendMessage(projectId, text, () => {});
        } catch (error) {
          console.error("Failed to send initial message:", error);
        }
      }, 100);
    } catch (error) {
      toast.error("创建项目失败：" + (error as Error).message);
    } finally {
      setIsLoading(false);
    }
  };

  const handleSelectProject = (project: Project) => {
    navigate(`/project/${project.id}`);
  };

  const quickActions = [
    
  ];

  return (
    <div className="flex h-screen w-full overflow-hidden bg-gradient-to-br from-background via-background to-muted/20">
      <main className="relative flex flex-1 flex-col overflow-hidden">
        <div className="flex-1 overflow-y-auto">
          <div className="flex flex-col min-h-full">
            {/* Hero section */}
            <div className="flex-1 flex flex-col items-center justify-center px-4 pt-12 pb-8">
              <div className="w-full max-w-[720px] text-center">
                {/* Animated icon */}
                <div className="mb-6 flex justify-center">
                  <div className="relative">
                    <div className="absolute inset-0 bg-gradient-to-r from-blue-500 via-purple-500 to-pink-500 rounded-full blur-2xl opacity-20 animate-pulse"></div>
                    <div className="relative bg-gradient-to-r from-blue-500 via-purple-500 to-pink-500 p-4 rounded-2xl shadow-2xl">
                      <Sparkles className="h-8 w-8 text-white" />
                    </div>
                  </div>
                </div>

                <h3 className="text-4xl md:text-4xl font-bold mb-3 bg-gradient-to-r from-foreground via-foreground/90 to-foreground/70 bg-clip-text text-transparent">
                  Having an idea?
                </h3>
                
                {/* Quick actions */}
                <div className="flex flex-wrap justify-center gap-3 mb-8">
                  {quickActions.map((action, index) => (
                    <button
                      key={index}
                      className="group flex items-center gap-2 px-4 py-2 rounded-full bg-muted/50 hover:bg-muted transition-all hover:scale-105"
                    >
                      <div className={`p-1.5 rounded-full bg-gradient-to-r ${action.color}`}>
                        <action.icon className="h-3.5 w-3.5 text-white" />
                      </div>
                      <span className="text-sm font-medium text-foreground/70 group-hover:text-foreground transition-colors">
                        {action.label}
                      </span>
                    </button>
                  ))}
                </div>

                {/* Input */}
                <div className="w-full relative">
                  <div className="absolute inset-0 bg-gradient-to-r from-blue-500/10 via-purple-500/10 to-pink-500/10 rounded-2xl blur-xl"></div>
                  <div className="relative">
                    <ChatInput onSend={handleSend} disabled={isLoading} />
                  </div>
                </div>

                <p className="mt-4 text-xs text-foreground/40">
                  按 Enter 发送，Shift + Enter 换行
                </p>
              </div>
            </div>

            {/* Projects section */}
            <div className="w-full px-4 md:px-8 pb-12">
              <div className="max-w-[1000px] mx-auto">
                <div className="relative rounded-3xl border border-border/50 bg-card/50 backdrop-blur-sm p-6 md:p-8 shadow-xl">
                  <div className="absolute inset-0 bg-gradient-to-br from-blue-500/5 via-transparent to-purple-500/5 rounded-3xl pointer-events-none"></div>
                  
                  <div className="relative">
                    {/* Tabs */}
                    <div className="flex items-center justify-between mb-8">
                      <div className="flex gap-6">
                        {["projects", "recent", "templates"].map((tab) => (
                          <button
                            key={tab}
                            onClick={() => setActiveTab(tab as any)}
                            className={`relative text-sm font-medium pb-2 transition-colors ${
                              activeTab === tab ? "text-foreground" : "text-foreground/60 hover:text-foreground"
                            }`}
                          >
                            {tab === "projects" ? "My projects" : tab === "recent" ? "Recently viewed" : "Templates"}
                            {activeTab === tab && (
                              <div className="absolute bottom-0 left-0 right-0 h-0.5 bg-gradient-to-r from-blue-500 to-purple-500 rounded-full"></div>
                            )}
                          </button>
                        ))}
                      </div>
                      <button className="flex items-center gap-2 text-sm font-medium text-foreground/60 hover:text-foreground transition-all hover:gap-3">
                        Browse all
                        <ArrowRight className="h-4 w-4" />
                      </button>
                    </div>

                    {/* Project cards */}
                    <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-5">
                      {projects.length === 0 ? (
                        <div className="col-span-full text-center py-12">
                          <div className="inline-flex items-center justify-center w-16 h-16 rounded-full bg-muted/50 mb-4">
                            <MessageSquare className="h-8 w-8 text-foreground/40" />
                          </div>
                          <p className="text-foreground/60 mb-2">还没有项目</p>
                          <p className="text-sm text-foreground/40">开始一个新对话来创建你的第一个项目</p>
                        </div>
                      ) : (
                        projects.map((project, index) => (
                          <div
                            key={project.id}
                            className="group cursor-pointer animate-in fade-in slide-in-from-bottom-4"
                            style={{ animationDelay: `${index * 50}ms`, animationFillMode: 'backwards' }}
                            onClick={() => handleSelectProject(project)}
                          >
                            <div className="relative aspect-[16/10] rounded-lg overflow-hidden mb-2 shadow-md group-hover:shadow-xl transition-all duration-300 group-hover:scale-[1.02]">
                              <div className={`absolute inset-0 ${project.thumbnail}`}></div>
                              <div className="absolute inset-0 bg-gradient-to-t from-black/20 to-transparent opacity-0 group-hover:opacity-100 transition-opacity"></div>
                              {project.badge && (
                                <div className="absolute top-2 left-2 px-2 py-0.5 bg-background/90 backdrop-blur-md rounded-full text-xs font-medium shadow-lg">
                                  {project.badge}
                                </div>
                              )}
                            </div>
                            
                            <div className="flex items-start gap-2">
                              <div className="flex-shrink-0 w-7 h-7 rounded-lg bg-gradient-to-br from-yellow-400 to-orange-500 flex items-center justify-center text-base shadow-sm group-hover:shadow-md transition-shadow">
                                {project.icon}
                              </div>
                              <div className="flex-1 min-w-0">
                                <h3 className="text-xs font-semibold text-foreground group-hover:text-blue-600 dark:group-hover:text-blue-400 transition-colors truncate">
                                  {project.name}
                                </h3>
                                <p className="text-xs text-foreground/50 mt-0.5">
                                  {project.editedTime}
                                </p>
                              </div>
                            </div>
                          </div>
                        ))
                      )}
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </main>
    </div>
  );
}
