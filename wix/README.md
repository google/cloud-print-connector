# Windows Installer

## Build Requirements
The WIX toolset is required to build the Windows Installer file. 
It can be downloaded from http://wixtoolset.org.

## Build Instructions
Build the Cloud Print Connector binaries.  See https://github.com/google/cloud-print-connector/wiki/Build-from-source

Update the dependencies.wxs file by running ./generate-dependencies.sh (in mingw64 bash shell).

Use the WIX tools to build the MSI.  The WIX tools that are used are candle.exe 
and light.exe.  They are installed by default to
"C:\Program Files (x86)\WiX Toolset v3.10\bin"
(/c/Program\ Files\ (x86)/WiX\ Toolset\ v3.10/bin/light.exe if you're using
mingw bash shell).  You can add this directory to your PATH to run the following
two commands.

Run candle.exe to build wixobj file from the wxs file:
```
candle.exe -arch x64 windows-connector.wxs dependencies.wxs
```

Expected output:
> Windows Installer XML Toolset Compiler version 3.10.2.2516
> Copyright (c) Outercurve Foundation. All rights reserved.
> 
> windows-connector.wxs
> dependencies.wxs


Run light.exe to build MSI file from the wixobj
```
light.exe -ext "C:\Program Files (x86)\WiX Toolset v3.10\bin\WixUIExtension.dll" windows-connector.wixobj dependencies.wixobj -o windows-connector.msi
```

Expected output:
> Windows Installer XML Toolset Linker version 3.10.2.2516
> Copyright (c) Outercurve Foundation. All rights reserved.

The light.exe command line requires the path of WixUIExtension.dll which 
provides the UI that is used by this installer.  If the WIX toolset is installed
to a different directory, use that directory path for the UI extension dll.

If the built Windows Connector binaries are not in $GOPATH\bin, then add -dSourceDir=<Path> 
to the light.exe command line to specify where the files can be found.

If mingw64 is not installed to C:\msys64\mingw64, then use -dDependencyDir=<Path> 
to specify where it is installed.

## Installation Instructions
Install the MSI by any normal method of installing an MSI file (double-clicking, automated deployment, etc.)

During an installation with UI, gcp-connector-util init will be run as the last step which 
will open a console window to initialize the connector.

The following public properties may be set during install of the MSI 
(see https://msdn.microsoft.com/en-us/library/windows/desktop/aa370912(v=vs.85).aspx) 
* CONFIGFILE = Path of connector config file to use instead of running gcp-connector-util init during install
