// Thin wrapper over the Wails bindings. When the page runs outside Wails
// (plain `vite dev` in a browser) it falls back to a scripted mock so the UI
// can be developed and previewed without the desktop shell.
import * as App from '../wailsjs/go/main/App';
import { EventsOn } from '../wailsjs/runtime/runtime';
import { createMock } from './mock';

const inWails = typeof window !== 'undefined' && !!window.go;
const mock = inWails ? null : createMock();

export const isMock = !inWails;

export const backend = inWails
  ? {
      sendMessage: App.SendMessage,
      cancel: App.Cancel,
      isRunning: App.IsRunning,
      resolveApproval: App.ResolveApproval,
      newChat: App.NewChat,
      currentChat: App.CurrentChat,
      listChats: App.ListChats,
      loadChat: App.LoadChat,
      deleteChat: App.DeleteChat,
      listSchedules: App.ListSchedules,
      addSchedule: App.AddSchedule,
      setScheduleEnabled: App.SetScheduleEnabled,
      deleteSchedule: App.DeleteSchedule,
      runScheduleNow: App.RunScheduleNow,
      getLaunchAtLogin: App.GetLaunchAtLogin,
      setLaunchAtLogin: App.SetLaunchAtLogin,
      listModels: App.ListModels,
      getStatus: App.GetStatus,
      getSettings: App.GetSettings,
      saveSettings: App.SaveSettings,
      startOllama: App.StartOllama,
      toolNames: App.ToolNames,
      paths: App.Paths,
      onEvent: (cb) => EventsOn('agent', cb),
    }
  : mock;
