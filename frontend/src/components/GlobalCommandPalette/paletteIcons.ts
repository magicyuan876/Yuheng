import type { Component } from "vue";

import {
  CircleHelpIcon,
  FileIcon,
  FolderIcon,
  HistoryIcon,
  MessageSquareIcon,
  MessageSquarePlusIcon,
  SearchIcon,
  SettingsIcon,
  UsersRoundIcon,
  XIcon,
} from "@lucide/vue";

/**
 * The palette's command catalogue speaks in tdesign icon names (it is data in
 * commands.ts). This is where those names meet the lucide set the palette
 * renders.
 */
export const PALETTE_ICONS: Record<string, Component> = {
  "chat-add": MessageSquarePlusIcon,
  chat: MessageSquareIcon,
  close: XIcon,
  file: FileIcon,
  folder: FolderIcon,
  "help-circle": CircleHelpIcon,
  history: HistoryIcon,
  search: SearchIcon,
  setting: SettingsIcon,
  usergroup: UsersRoundIcon,
};

/** The icon for a catalogue name, or the fallback when the name is unknown. */
export function paletteIcon(name: string | undefined, fallback: Component): Component {
  return (name && PALETTE_ICONS[name]) || fallback;
}
