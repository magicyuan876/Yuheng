// TDesign's globally installed components, registered with the template
// type-checker — deliberately untyped.
//
// `checkUnknownComponents` (tsconfig.app.json) makes a template that uses a
// component it never imported a type error, which is a bug the toolchain
// used to let through silently: the tag rendered nothing and the console got
// a warning nobody reads. For that check to work, every component installed
// globally by `app.use(TDesign)` has to be known to the checker, and this file
// is what makes them known.
//
// They are `any` on purpose. TDesign ships real types for all of them
// (`tdesign-vue-next/global`), and loading those instead surfaces 78 prop-type
// errors in inherited code — wrong placement literals, handler signatures
// that do not match — which are real and should be fixed, but not as a side
// effect of turning a different check on. When they are, replace this file
// with `"types": ["vite/client", "tdesign-vue-next/global"]` and delete it.
// Do not load both: the two declarations of each component would conflict.
//
// Generated from node_modules/tdesign-vue-next/global.d.ts (121 components).
// Regenerate after a TDesign upgrade if a template's new <t-*> tag is reported
// as unknown.

declare module "vue" {
  export interface GlobalComponents {
    TAffix: any;
    TAlert: any;
    TAnchor: any;
    TAnchorItem: any;
    TAnchorTarget: any;
    TAside: any;
    TAutoComplete: any;
    TAvatar: any;
    TAvatarGroup: any;
    TBackTop: any;
    TBadge: any;
    TBaseTable: any;
    TBreadcrumb: any;
    TBreadcrumbItem: any;
    TButton: any;
    TCalendar: any;
    TCard: any;
    TCascader: any;
    TCheckbox: any;
    TCheckboxGroup: any;
    TCheckTag: any;
    TCheckTagGroup: any;
    TCol: any;
    TCollapse: any;
    TCollapsePanel: any;
    TColorPicker: any;
    TColorPickerPanel: any;
    TComment: any;
    TConfigProvider: any;
    TContent: any;
    TDatePicker: any;
    TDatePickerPanel: any;
    TDateRangePicker: any;
    TDateRangePickerPanel: any;
    TDescriptions: any;
    TDescriptionsItem: any;
    TDialog: any;
    TDialogCard: any;
    TDivider: any;
    TDrawer: any;
    TDropdown: any;
    TDropdownItem: any;
    TDropdownMenu: any;
    TEmpty: any;
    TEnhancedTable: any;
    TFooter: any;
    TForm: any;
    TFormItem: any;
    TGuide: any;
    THeader: any;
    THeadMenu: any;
    TIcon: any;
    TImage: any;
    TImageViewer: any;
    TInput: any;
    TInputAdornment: any;
    TInputGroup: any;
    TInputNumber: any;
    TLayout: any;
    TLink: any;
    TList: any;
    TListItem: any;
    TListItemMeta: any;
    TLoading: any;
    TMenu: any;
    TMenuGroup: any;
    TMenuItem: any;
    TMessage: any;
    TNotification: any;
    TOption: any;
    TOptionGroup: any;
    TPagination: any;
    TPaginationMini: any;
    TTypographyParagraph: any;
    TPopconfirm: any;
    TPopup: any;
    TPrimaryTable: any;
    TProgress: any;
    TQrcode: any;
    TRadio: any;
    TRadioButton: any;
    TRadioGroup: any;
    TRangeInput: any;
    TRangeInputPopup: any;
    TRate: any;
    TRow: any;
    TSearch: any;
    TSelect: any;
    TSelectInput: any;
    TSkeleton: any;
    TSlider: any;
    TSpace: any;
    TStatistic: any;
    TStepItem: any;
    TSteps: any;
    TStickyItem: any;
    TStickyTool: any;
    TSubmenu: any;
    TSwiper: any;
    TSwiperItem: any;
    TSwitch: any;
    TTable: any;
    TTabPanel: any;
    TTabs: any;
    TTag: any;
    TTagInput: any;
    TTypographyText: any;
    TTextarea: any;
    TTimeline: any;
    TTimelineItem: any;
    TTimePicker: any;
    TTimeRangePicker: any;
    TTypographyTitle: any;
    TTooltip: any;
    TTooltipLite: any;
    TTransfer: any;
    TTree: any;
    TTreeSelect: any;
    TTypography: any;
    TUpload: any;
    TWatermark: any;
  }
}

export {};
