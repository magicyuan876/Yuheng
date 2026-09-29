// Maps the TDesign icon names returned by getFileIcon() (utils/files) to
// lucide components for the migrated (Tailwind / shadcn-vue) knowledge views.
import {
  CodeIcon,
  FileAudioIcon,
  FileIcon,
  FileSpreadsheetIcon,
  FileTextIcon,
  FileVideoIcon,
  ImageIcon,
  LinkIcon,
  PenLineIcon,
  PresentationIcon,
  type LucideIcon,
} from "@lucide/vue";

const fileTypeIconMap: Record<string, LucideIcon> = {
  edit: PenLineIcon,
  link: LinkIcon,
  file: FileIcon,
  "file-pdf": FileTextIcon,
  "file-word": FileTextIcon,
  "file-excel": FileSpreadsheetIcon,
  "file-powerpoint": PresentationIcon,
  code: CodeIcon,
  image: ImageIcon,
  sound: FileAudioIcon,
  video: FileVideoIcon,
};

export function fileTypeIcon(name: string): LucideIcon {
  return fileTypeIconMap[name] ?? FileIcon;
}
