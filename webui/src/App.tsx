import React from 'react';
import { BrowserRouter as Router, Routes, Route } from 'react-router-dom';
import {
  Page,
  PageSidebar,
  Nav,
  NavList,
  NavItem,
  Brand,
  PageSidebarBody,
  Masthead,
  MastheadMain,
  MastheadBrand,
  MastheadToggle,
  PageToggleButton,
  SkipToContent,
  BackToTop,
  Title
} from '@patternfly/react-core';
import { CubesIcon, ListIcon, BarsIcon, CatalogIcon } from '@patternfly/react-icons';
import CreateBuildPage from './components/CreateBuildPage';
import BuildListPage from './components/BuildListPage';
import CatalogPage from './components/CatalogPage';

const App: React.FC = () => {
  const [isSidebarOpen, setIsSidebarOpen] = React.useState(true);
  const mainContentId = "main-content";

  const onSidebarToggle = () => {
    setIsSidebarOpen(!isSidebarOpen);
  };

  const masthead = (
    <Masthead>
      <MastheadToggle>
        <PageToggleButton
          variant="plain"
          aria-label="Global navigation"
          isSidebarOpen={isSidebarOpen}
          onSidebarToggle={onSidebarToggle}
        >
          <BarsIcon />
        </PageToggleButton>
      </MastheadToggle>
      <MastheadMain>
        <MastheadBrand>
          <Brand alt="AIB UI">
            <Title headingLevel="h1" size="lg">AIB UI</Title>
          </Brand>
        </MastheadBrand>
      </MastheadMain>
    </Masthead>
  );

  const Navigation = (
    <Nav>
      <NavList>
        <NavItem to="/create" isActive={window.location.pathname === '/create'}>
          <CubesIcon /> Create Build
        </NavItem>
        <NavItem to="/builds" isActive={window.location.pathname === '/builds'}>
          <ListIcon /> Build List
        </NavItem>
        <NavItem to="/catalog" isActive={window.location.pathname === '/catalog'}>
          <CatalogIcon /> Catalog
        </NavItem>
      </NavList>
    </Nav>
  );

  const sidebar = (
    <PageSidebar isSidebarOpen={isSidebarOpen}>
      <PageSidebarBody>
        {Navigation}
      </PageSidebarBody>
    </PageSidebar>
  );

  return (
    <Router>
      <Page 
        masthead={masthead} 
        sidebar={sidebar}
        mainContainerId={mainContentId}
        skipToContent={<SkipToContent href={`#${mainContentId}`}>Skip to content</SkipToContent>}
      >
        <BackToTop scrollableSelector={`#${mainContentId}`} />
        <Routes>
          <Route path="/" element={<BuildListPage />} />
          <Route path="/builds" element={<BuildListPage />} />
          <Route path="/create" element={<CreateBuildPage />} />
          <Route path="/catalog" element={<CatalogPage />} />
        </Routes>
      </Page>
    </Router>
  );
};

export default App;