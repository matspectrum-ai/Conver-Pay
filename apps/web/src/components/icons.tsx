import type { SVGProps } from "react";

type IconProps = SVGProps<SVGSVGElement>;

function Icon({ children, ...props }: IconProps & { children: React.ReactNode }) {
  return <svg viewBox="0 0 20 20" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true" {...props}>{children}</svg>;
}

export const Icons = {
  overview: (p: IconProps) => <Icon {...p}><path d="M3.5 10.5h5v6h-5zM11.5 3.5h5v13h-5zM3.5 3.5h5v4h-5z" /></Icon>,
  payments: (p: IconProps) => <Icon {...p}><path d="M3.5 5.5h13v9h-13z"/><path d="M6 9.5h4M13.7 10h.1"/></Icon>,
  recoveries: (p: IconProps) => <Icon {...p}><path d="M15.7 6.2A6.5 6.5 0 1 0 16 13"/><path d="M15.8 3v3.5h-3.5"/><path d="M7.5 10h5M10 7.5v5"/></Icon>,
  routing: (p: IconProps) => <Icon {...p}><circle cx="5" cy="5" r="1.8"/><circle cx="15" cy="5" r="1.8"/><circle cx="10" cy="15" r="1.8"/><path d="M6.6 5.8 9.1 13M13.4 5.8 10.9 13M6.8 5h6.4"/></Icon>,
  providers: (p: IconProps) => <Icon {...p}><rect x="3.5" y="3.5" width="5" height="5" rx="1"/><rect x="11.5" y="3.5" width="5" height="5" rx="1"/><rect x="7.5" y="11.5" width="5" height="5" rx="1"/><path d="M6 8.5v1.2h8V8.5M10 9.7v1.8"/></Icon>,
  observe: (p: IconProps) => <Icon {...p}><path d="M2.8 10s2.6-4.5 7.2-4.5S17.2 10 17.2 10 14.6 14.5 10 14.5 2.8 10 2.8 10Z"/><circle cx="10" cy="10" r="2"/></Icon>,
  webhooks: (p: IconProps) => <Icon {...p}><path d="M6.2 6.7a3 3 0 1 1 2.4-3"/><path d="M13.8 6.7a3 3 0 1 0-2.4-3"/><path d="M7.2 12.2a3 3 0 1 0 5.6 0"/><path d="m7.8 7.2 2.2 4.3 2.2-4.3"/></Icon>,
  developers: (p: IconProps) => <Icon {...p}><path d="m7.5 6-4 4 4 4M12.5 6l4 4-4 4M11 4l-2 12"/></Icon>,
  settings: (p: IconProps) => <Icon {...p}><circle cx="10" cy="10" r="2.4"/><path d="M10 2.8v2M10 15.2v2M17.2 10h-2M4.8 10h-2M15.1 4.9l-1.4 1.4M6.3 13.7l-1.4 1.4M15.1 15.1l-1.4-1.4M6.3 6.3 4.9 4.9"/></Icon>,
  search: (p: IconProps) => <Icon {...p}><circle cx="8.8" cy="8.8" r="4.8"/><path d="m12.4 12.4 4 4"/></Icon>,
  plus: (p: IconProps) => <Icon {...p}><path d="M10 4v12M4 10h12"/></Icon>,
  chevron: (p: IconProps) => <Icon {...p}><path d="m7 8 3 3 3-3"/></Icon>,
  arrow: (p: IconProps) => <Icon {...p}><path d="M4 10h11M11 6l4 4-4 4"/></Icon>,
  command: (p: IconProps) => <Icon {...p}><path d="M7 6.5H5.5a2 2 0 1 1 2-2V16a2 2 0 1 1-2-2h9a2 2 0 1 1-2 2V4.5a2 2 0 1 1 2 2H7Z"/></Icon>,
};
