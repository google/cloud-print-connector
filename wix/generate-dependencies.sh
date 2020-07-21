#!/usr/bin/bash
echo '<Wix xmlns="http://schemas.microsoft.com/wix/2006/wi">'>dependencies.wxs
echo '<Fragment>
    <!-- Group of components of all the dependency dlls -->
    <ComponentGroup
        Id="Dependencies"
        Directory="INSTALLFOLDER"
        Source="!(wix.DependencyDir)">'>>dependencies.wxs
for f in `ldd ${GOPATH}/bin/gcp-windows-connector.exe | grep -i -v Windows | sed s/" =>.*"// | sed s/"\t"// | sort`
  do echo "      <Component>
        <File Name=\"$f\" KeyPath=\"yes\"/>
      </Component>">>dependencies.wxs; done
echo '    </ComponentGroup>
  </Fragment>
</Wix>'>>dependencies.wxs

